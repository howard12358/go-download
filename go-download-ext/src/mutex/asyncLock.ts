class AsyncLock {
    private queue: (() => void)[] = []
    private locked = false

    async acquire(): Promise<() => void> {
        return new Promise(resolve => {
            const unlock = () => {
                this.locked = false
                const next = this.queue.shift()
                if (next) {
                    this.locked = true
                    next()
                }
            }

            if (!this.locked) {
                this.locked = true
                resolve(unlock)
            } else {
                this.queue.push(() => resolve(unlock))
            }
        })
    }
}

export const historyLock = new AsyncLock()
