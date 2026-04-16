import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Clock } from 'lucide-react'
import { TIME_RANGE_LABELS, type TimeRange } from '@/types'

interface TimeRangePickerProps {
  value: TimeRange
  onChange: (value: TimeRange) => void
}

export function TimeRangePicker({ value, onChange }: TimeRangePickerProps) {
  return (
    <Select value={value} onValueChange={(v) => onChange(v as TimeRange)}>
      <SelectTrigger className="w-44">
        <Clock className="mr-2 h-4 w-4 text-muted-foreground" />
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {Object.entries(TIME_RANGE_LABELS).map(([key, label]) => (
          key !== 'custom' && (
            <SelectItem key={key} value={key}>
              {label}
            </SelectItem>
          )
        ))}
      </SelectContent>
    </Select>
  )
}
