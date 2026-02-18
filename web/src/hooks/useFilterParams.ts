import { useState, useEffect, useCallback } from 'react'

/**
 * Hook to persist filter state to URL search params.
 * Works without React Router by using `window.location.search` + `history.replaceState`.
 * Now supports cross-component synchronization via custom event.
 */

const EVENT_NAME = 'argus:urlchange'

function getParams(): URLSearchParams {
    return new URLSearchParams(window.location.search)
}

function setParams(params: URLSearchParams) {
    const search = params.toString()
    const url = search ? `${window.location.pathname}?${search}` : window.location.pathname
    window.history.replaceState(null, '', url)
    window.dispatchEvent(new Event(EVENT_NAME))
}

/**
 * Sync a single URL param key with React state.
 * - `defaultValue` is used when the param is absent from the URL.
 * - Setting `null` or `''` removes the param from the URL.
 */
export function useFilterParam(key: string, defaultValue: string | null): [string | null, (v: string | null) => void] {
    const [value, setValue] = useState<string | null>(() => {
        const p = getParams().get(key)
        return p !== null ? p : defaultValue
    })

    const setAndPersist = useCallback((newValue: string | null) => {
        const params = getParams()
        if (newValue && newValue !== defaultValue) {
            params.set(key, newValue)
        } else {
            params.delete(key)
        }
        setParams(params)
    }, [key, defaultValue])

    useEffect(() => {
        const onUrlChange = () => {
            const p = getParams().get(key)
            const nextValue = p !== null ? p : defaultValue
            setValue(nextValue)
        }

        window.addEventListener(EVENT_NAME, onUrlChange)
        window.addEventListener('popstate', onUrlChange)
        return () => {
            window.removeEventListener(EVENT_NAME, onUrlChange)
            window.removeEventListener('popstate', onUrlChange)
        }
    }, [key, defaultValue])

    return [value, setAndPersist]
}

/**
 * Like useFilterParam but for non-nullable string values (always has a valid default).
 */
export function useFilterParamString(key: string, defaultValue: string): [string, (v: string) => void] {
    const [value, setValue] = useState<string>(() => {
        const p = getParams().get(key)
        return p !== null && p !== '' ? p : defaultValue
    })

    const setAndPersist = useCallback((newValue: string) => {
        const params = getParams()
        if (newValue && newValue !== defaultValue) {
            params.set(key, newValue)
        } else {
            params.delete(key)
        }
        setParams(params)
    }, [key, defaultValue])

    useEffect(() => {
        const onUrlChange = () => {
            const p = getParams().get(key)
            const nextValue = p !== null && p !== '' ? p : defaultValue
            setValue(nextValue)
        }

        window.addEventListener(EVENT_NAME, onUrlChange)
        window.addEventListener('popstate', onUrlChange)
        return () => {
            window.removeEventListener(EVENT_NAME, onUrlChange)
            window.removeEventListener('popstate', onUrlChange)
        }
    }, [key, defaultValue])

    return [value, setAndPersist]
}
