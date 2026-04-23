import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import '@mantine/core/styles.css'
import { MantineProvider, createTheme } from '@mantine/core'
import './styles/tokens.css'
import './styles/global.css'
import App from './App'

const theme = createTheme({
  primaryColor: 'blue',
  defaultRadius: 'md',
  fontFamily:
    'system-ui, -apple-system, "Segoe UI", Roboto, Oxygen, Ubuntu, Cantarell, "Open Sans", "Helvetica Neue", sans-serif',
  fontFamilyMonospace: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
})

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <MantineProvider theme={theme} defaultColorScheme="dark">
      <App />
    </MantineProvider>
  </StrictMode>,
)
