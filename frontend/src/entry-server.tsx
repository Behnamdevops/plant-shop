import { Writable } from 'node:stream'
import { renderToPipeableStream } from 'react-dom/server'
import { StaticRouter } from 'react-router-dom'
import { AppContent } from './App'
import { PublicDataContext } from './context/PublicDataContext'
import type { PublicData } from './context/PublicDataContext'
export function render(path: string, data: PublicData): Promise<string> {
  return new Promise((resolve, reject) => {
    let html = '',
      failed = false
    const output = new Writable({
      write(chunk, _encoding, callback) {
        html += chunk.toString()
        callback()
      },
    })
    const stream = renderToPipeableStream(
      <StaticRouter location={path}>
        <PublicDataContext.Provider value={data}>
          <AppContent />
        </PublicDataContext.Provider>
      </StaticRouter>,
      {
        onAllReady() {
          stream.pipe(output)
        },
        onShellError: reject,
        onError() {
          failed = true
        },
      },
    )
    const timer = setTimeout(() => {
      stream.abort()
      reject(new Error('SSR timeout'))
    }, 8000)
    output.on('finish', () => {
      clearTimeout(timer)
      if (failed) reject(new Error('SSR failed'))
      else resolve(html)
    })
    output.on('error', (error) => {
      clearTimeout(timer)
      reject(error)
    })
  })
}
