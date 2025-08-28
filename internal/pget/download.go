package pget

import (
	"context"
	"fmt"
	"golang.org/x/sync/errgroup"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
)

type assignTasksConfig struct {
	Procs         int
	TaskSize      int64 // download filesize per task
	ContentLength int64 // full download filesize
	URLs          []string
	PartialDir    string
	Filename      string
	Client        *http.Client
}

type task struct {
	ID         int
	Procs      int
	URL        string
	Range      Range
	PartialDir string
	Filename   string
	Client     *http.Client
}

func (t *task) destPath() string {
	return getPartialFilePath(t.PartialDir, t.Filename, t.Procs, t.ID)
}

func (t *task) String() string {
	return fmt.Sprintf("task[%d]: %q", t.ID, t.destPath())
}

type makeRequestOption struct {
	useragent string
	referer   string
}

func (t *task) makeRequest(ctx context.Context, opt *makeRequestOption) (*http.Request, error) {
	req, err := http.NewRequest("GET", t.URL, nil)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to make a new request: %d", t.ID))
	}
	req = req.WithContext(ctx)

	// set download ranges
	req.Header.Set("Range", t.Range.BytesRange())

	// set useragent
	req.Header.Set("User-Agent", opt.useragent)

	// set referer
	if opt.referer != "" {
		req.Header.Set("Referer", opt.referer)
	}

	return req, nil
}

// assignTasks creates task to assign it to each goroutines
func assignTasks(c *assignTasksConfig) []*task {
	tasks := make([]*task, 0, c.Procs)

	var totalActiveProcs int
	for i := 0; i < c.Procs; i++ {

		r := makeRange(i, c.Procs, c.TaskSize, c.ContentLength)

		partName := getPartialFilePath(c.PartialDir, c.Filename, c.Procs, i)

		if info, err := os.Stat(partName); err == nil {
			infosize := info.Size()
			// check if the part is fully downloaded
			if i == c.Procs-1 {
				if infosize == r.high-r.low {
					continue
				}
			} else if infosize == c.TaskSize {
				// skip as the part is already downloaded
				continue
			}

			// make low range from this next byte
			r.low += infosize
		}

		tasks = append(tasks, &task{
			ID:         i,
			Procs:      c.Procs,
			URL:        c.URLs[totalActiveProcs%len(c.URLs)],
			Range:      r,
			PartialDir: c.PartialDir,
			Filename:   c.Filename,
			Client:     c.Client,
		})

		totalActiveProcs++
	}

	return tasks
}

type DownloadConfig struct {
	Filename      string
	Dirname       string
	ContentLength int64
	Procs         int
	URLs          []string
	Client        *http.Client

	*makeRequestOption

	ProgressFn ProgressFunc
}

type DownloadOption func(c *DownloadConfig)

// WithProgressCallback 把进度回调注入到 Pget
func WithProgressCallback(fn ProgressFunc) DownloadOption {
	return func(c *DownloadConfig) {
		c.ProgressFn = fn
	}
}

func WithUserAgent(ua, version string) DownloadOption {
	return func(c *DownloadConfig) {
		if ua == "" {
			ua = "Pget/" + version
		}
		c.makeRequestOption.useragent = ua
	}
}

func WithReferer(referer string) DownloadOption {
	return func(c *DownloadConfig) {
		c.makeRequestOption.referer = referer
	}
}

func Download(ctx context.Context, c *DownloadConfig, opts ...DownloadOption) error {
	partialDir := getPartialDirname(c.Dirname, c.Filename, c.Procs)

	// create download location
	if err := os.MkdirAll(partialDir, 0755); err != nil {
		return errors.Wrap(err, "failed to mkdir for download location")
	}

	c.makeRequestOption = &makeRequestOption{}

	for _, opt := range opts {
		opt(c)
	}

	tasks := assignTasks(&assignTasksConfig{
		Procs:         c.Procs,
		TaskSize:      c.ContentLength / int64(c.Procs),
		ContentLength: c.ContentLength,
		URLs:          c.URLs,
		PartialDir:    partialDir,
		Filename:      c.Filename,
		Client:        newClient(c.Client),
	})

	if err := parallelDownload(ctx, &parallelDownloadConfig{
		ContentLength:     c.ContentLength,
		Tasks:             tasks,
		PartialDir:        partialDir,
		makeRequestOption: c.makeRequestOption,
		DownloadConfig:    c,
	}); err != nil {
		log.Println("parallelDownload failed", err)
		return err
	}

	return bindFiles(c, partialDir)
}

type parallelDownloadConfig struct {
	ContentLength int64
	Tasks         []*task
	PartialDir    string
	*makeRequestOption

	*DownloadConfig
}

func parallelDownload(ctx context.Context, c *parallelDownloadConfig) error {
	eg, ctx := errgroup.WithContext(ctx)

	// check file size already downloaded for resume
	_, err := checkProgress(c.PartialDir)
	if err != nil {
		return errors.Wrap(err, "failed to get directory size")
	}

	// 全局累计已下载字节
	var downloaded int64

	// 启动采样器，定时计算下载速度
	sampleInterval := 1500 * time.Millisecond
	if c.DownloadConfig != nil && c.DownloadConfig.ProgressFn != nil {
		// 使用一个 goroutine 周期性计算 delta / 秒上报 speed
		eg.Go(func() error {
			ticker := time.NewTicker(sampleInterval)
			defer ticker.Stop()
			last := int64(0)
			lastTime := time.Now()
			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case t := <-ticker.C:
					cur := atomic.LoadInt64(&downloaded)
					delta := cur - last
					elapsed := t.Sub(lastTime).Seconds()
					speed := int64(0)
					if delta > 0 && elapsed > 0 {
						speed = int64(float64(delta) / elapsed)
					}
					c.DownloadConfig.ProgressFn(cur, c.ContentLength, speed)
					// 如果已经完成，退出采样器
					if c.ContentLength > 0 && cur >= c.ContentLength {
						return nil
					}
					last = cur
					lastTime = t
				}
			}
		})
	}

	for _, task := range c.Tasks {
		task := task
		eg.Go(func() error {
			req, err := task.makeRequest(ctx, c.makeRequestOption)
			if err != nil {
				return err
			}
			return task.downloadWithProgress(req, &downloaded, c.ContentLength, c.DownloadConfig.ProgressFn)
		})
	}

	err = eg.Wait()

	// 最后确保上报 100% 且 speed=0
	if c.DownloadConfig != nil && c.DownloadConfig.ProgressFn != nil {
		cur := atomic.LoadInt64(&downloaded)
		c.DownloadConfig.ProgressFn(cur, c.ContentLength, 0)
	}

	return err
}

func (t *task) downloadWithProgress(
	req *http.Request,
	downloaded *int64,
	total int64,
	progressFn ProgressFunc,
) error {
	resp, err := t.Client.Do(req)
	if err != nil {
		return errors.Wrapf(err, "failed to get response: %q", t.String())
	}
	defer resp.Body.Close()

	f, err := os.OpenFile(t.destPath(), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return errors.Wrapf(err, "failed to create: %q", t.String())
	}
	defer f.Close()

	buf := make([]byte, 32*1024)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := f.Write(buf[:n]); writeErr != nil {
				return errors.Wrapf(writeErr, "write error: %q", t.String())
			}
			// 原子累加，并触发回调
			newTotal := atomic.AddInt64(downloaded, int64(n))
			if progressFn != nil {
				// 只上报下载进度，不上报速度
				progressFn(newTotal, total, -1)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return errors.Wrapf(readErr, "read error: %q", t.String())
		}
	}
	return nil
}

//func parallelDownload(ctx context.Context, c *parallelDownloadConfig) error {
//	eg, ctx := errgroup.WithContext(ctx)
//
//	bar := pb.Start64(c.ContentLength).SetWriter(stdout).Set(pb.Bytes, true)
//	defer bar.Finish()
//
//	// check file size already downloaded for resume
//	size, err := checkProgress(c.PartialDir)
//	if err != nil {
//		return errors.Wrap(err, "failed to get directory size")
//	}
//
//	bar.SetCurrent(size)
//
//	for _, task := range c.Tasks {
//		task := task
//		eg.Go(func() error {
//			req, err := task.makeRequest(ctx, c.makeRequestOption)
//			if err != nil {
//				return err
//			}
//			return task.download(req, bar)
//		})
//	}
//
//	return eg.Wait()
//}

//func (t *task) download(req *http.Request, bar *pb.ProgressBar) error {
//	resp, err := t.Client.Do(req)
//	if err != nil {
//		return errors.Wrapf(err, "failed to get response: %q", t.String())
//	}
//	defer resp.Body.Close()
//
//	output, err := os.OpenFile(t.destPath(), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
//	if err != nil {
//		return errors.Wrapf(err, "failed to create: %q", t.String())
//	}
//	defer output.Close()
//
//	//rd := bar.NewProxyReader(resp.Body)
//
//	//if _, err := io.Copy(output, rd); err != nil {
//	if _, err := io.Copy(output, resp.Body); err != nil {
//		return errors.Wrapf(err, "failed to write response body: %q", t.String())
//	}
//
//	return nil
//}

func bindFiles(c *DownloadConfig, partialDir string) error {
	//log.Println("start bind files, target file:", c.Dirname+c.Filename)

	destPath := filepath.Join(c.Dirname, c.Filename)
	f, err := os.Create(destPath)
	if err != nil {
		return errors.Wrap(err, "failed to create a file in download location")
	}
	defer f.Close()

	//bar := pb.Start64(c.ContentLength).SetWriter(stdout)

	copyFn := func(name string) error {
		subfp, err := os.Open(name)
		if err != nil {
			return errors.Wrapf(err, "failed to open %q in download location", name)
		}

		defer subfp.Close()

		//proxy := bar.NewProxyReader(subfp)
		if _, err := io.Copy(f, subfp); err != nil {
			return errors.Wrapf(err, "failed to copy %q", name)
		}

		return nil
	}

	for i := 0; i < c.Procs; i++ {
		partialFilename := getPartialFilePath(partialDir, c.Filename, c.Procs, i)
		if err := copyFn(partialFilename); err != nil {
			return err
		}

		// remove a file in download location for join
		if err := os.Remove(partialFilename); err != nil {
			return errors.Wrapf(err, "failed to remove %q in download location", partialFilename)
		}
	}

	//bar.Finish()

	// remove download location
	// RemoveAll reason: will create .DS_Store in download location if execute on mac
	if err := os.RemoveAll(partialDir); err != nil {
		return errors.Wrap(err, "failed to remove download location")
	}

	return nil
}
