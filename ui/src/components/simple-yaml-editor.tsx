import { IconTextWrap } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'

import { MonacoEditor } from '@/lib/monaco-loader'
import {
  defineMonacoBackgroundThemes,
  useMonacoBackgroundColor,
} from '@/lib/monaco-theme'
import { cn } from '@/lib/utils'
import { useWordWrap } from '@/hooks/use-word-wrap'
import { Button } from '@/components/ui/button'

import { useAppearance } from './appearance-provider'
import { Tooltip, TooltipContent, TooltipTrigger } from './ui/tooltip'

interface SimpleYamlEditorProps {
  value: string
  onChange: (value: string | undefined) => void
  disabled?: boolean
  height?: string
}

export function SimpleYamlEditor({
  value,
  onChange,
  disabled = false,
  height = '400px',
}: SimpleYamlEditorProps) {
  const { t } = useTranslation()
  const { actualTheme, colorTheme } = useAppearance()
  const themeMode = actualTheme === 'dark' ? 'dark' : 'light'
  const backgroundColor = useMonacoBackgroundColor(
    '--background',
    themeMode,
    colorTheme
  )
  const { wordWrap, toggleWordWrap } = useWordWrap()

  return (
    <div className="relative border rounded-md overflow-hidden">
      <Tooltip>
        <TooltipTrigger asChild>
          <Button
            variant="ghost"
            size="icon"
            className={cn(
              'absolute right-2 top-2 z-10 h-7 w-7 bg-background/80 backdrop-blur',
              wordWrap === 'on' && 'bg-accent text-accent-foreground'
            )}
            onClick={toggleWordWrap}
          >
            <IconTextWrap className="h-4 w-4" />
          </Button>
        </TooltipTrigger>
        <TooltipContent>{t('common.actions.toggleWordWrap')}</TooltipContent>
      </Tooltip>
      <MonacoEditor
        key={`simple-yaml-editor-${colorTheme}-${actualTheme}-${backgroundColor}`}
        height={height}
        defaultLanguage="yaml"
        value={value}
        onChange={onChange}
        loading={
          <div
            className="flex items-center justify-center h-full text-muted-foreground"
            style={{ height }}
          >
            Loading editor...
          </div>
        }
        beforeMount={(monaco) => {
          defineMonacoBackgroundThemes(monaco, {
            darkThemeName: `custom-dark-${colorTheme}`,
            lightThemeName: `custom-vs-${colorTheme}`,
            backgroundColor,
          })
        }}
        theme={
          actualTheme === 'dark'
            ? `custom-dark-${colorTheme}`
            : `custom-vs-${colorTheme}`
        }
        options={{
          minimap: { enabled: false },
          scrollBeyondLastLine: false,
          wordWrap,
          readOnly: disabled,
          fontSize: 14,
          lineNumbers: 'on',
          folding: true,
          autoIndent: 'full',
          formatOnPaste: true,
          formatOnType: true,
          tabSize: 2,
          insertSpaces: true,
          detectIndentation: true,
          renderWhitespace: 'boundary',
          scrollbar: {
            verticalScrollbarSize: 8,
            horizontalScrollbarSize: 8,
          },
          fontFamily:
            "'Maple Mono', Monaco, 'Cascadia Code', 'Roboto Mono', Consolas, 'Courier New', monospace",
        }}
      />
    </div>
  )
}
