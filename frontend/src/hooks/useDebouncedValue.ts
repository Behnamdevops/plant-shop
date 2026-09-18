import { useEffect, useState } from 'react'

// useDebouncedValue returns a copy of `value` that only updates after
// `delayMs` has elapsed without `value` changing again. Used to avoid
// firing a network request on every keystroke in the catalog search box.
// No external dependency needed — just setTimeout + cleanup.
export function useDebouncedValue<T>(value: T, delayMs: number): T {
  const [debounced, setDebounced] = useState(value)

  useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), delayMs)
    return () => clearTimeout(timer)
  }, [value, delayMs])

  return debounced
}
