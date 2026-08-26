import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'

export function UserDisplayName({
  name,
  login,
}: {
  name?: string
  login: string
}) {
  if (!name || name === login) {
    return <span>{login}</span>
  }
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span>{name}</span>
      </TooltipTrigger>
      <TooltipContent>{login}</TooltipContent>
    </Tooltip>
  )
}
