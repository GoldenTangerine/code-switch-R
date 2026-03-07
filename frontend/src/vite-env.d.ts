/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_SETTINGS_PERSIST_DEBOUNCE_MS?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
