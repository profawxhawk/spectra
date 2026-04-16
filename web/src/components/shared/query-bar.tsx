import { useState } from 'react'
import { Search } from 'lucide-react'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'

interface QueryBarProps {
  onSearch: (query: string) => void
  placeholder?: string
  defaultValue?: string
}

export function QueryBar({ onSearch, placeholder = 'Search traces...', defaultValue = '' }: QueryBarProps) {
  const [value, setValue] = useState(defaultValue)

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (value.trim()) onSearch(value.trim())
  }

  return (
    <form onSubmit={handleSubmit} className="flex gap-2">
      <div className="relative flex-1">
        <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          value={value}
          onChange={(e) => setValue(e.target.value)}
          placeholder={placeholder}
          className="pl-9"
        />
      </div>
      <Button type="submit" size="default">
        Search
      </Button>
    </form>
  )
}
