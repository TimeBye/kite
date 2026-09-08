import { useCallback, useState } from 'react'

const STORAGE_KEY = 'yaml-editor-word-wrap'

export function useWordWrap() {
  const [wordWrap, setWordWrap] = useState<'on' | 'off'>(() =>
    localStorage.getItem(STORAGE_KEY) === 'off' ? 'off' : 'on'
  )

  const toggleWordWrap = useCallback(() => {
    setWordWrap((prev) => {
      const next = prev === 'on' ? 'off' : 'on'
      localStorage.setItem(STORAGE_KEY, next)
      return next
    })
  }, [])

  return { wordWrap, toggleWordWrap }
}
