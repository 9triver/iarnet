"use client"

import { useState, useEffect, useRef } from "react"
import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { Calendar } from "@/components/ui/calendar"
import { format } from "date-fns"
import { zhCN } from "date-fns/locale"
import { Clock, Calendar as CalendarIcon } from "lucide-react"

// 滚动时间选择器组件
function TimePicker({
  value,
  onChange,
  minTime,
  maxTime,
}: {
  value: string
  onChange: (value: string) => void
  minTime?: string
  maxTime?: string
}) {
  const [hours, minutes] = value ? value.split(":") : ["00", "00"]
  const selectedHour = parseInt(hours, 10)
  const selectedMinute = parseInt(minutes, 10)

  const minHour = minTime ? parseInt(minTime.split(":")[0], 10) : 0
  const minMinute = minTime ? parseInt(minTime.split(":")[1], 10) : 0
  const maxHour = maxTime ? parseInt(maxTime.split(":")[0], 10) : 23
  const maxMinute = maxTime ? parseInt(maxTime.split(":")[1], 10) : 59

  const hoursList = Array.from({ length: 24 }, (_, i) => i)
  const minutesList = Array.from({ length: 60 }, (_, i) => i)

  const hourScrollRef = useRef<HTMLDivElement>(null)
  const minuteScrollRef = useRef<HTMLDivElement>(null)

  // 自动滚动到选中的值
  useEffect(() => {
    if (hourScrollRef.current) {
      const selectedElement = hourScrollRef.current.children[selectedHour] as HTMLElement
      if (selectedElement) {
        selectedElement.scrollIntoView({ block: "center", behavior: "smooth" })
      }
    }
  }, [selectedHour])

  useEffect(() => {
    if (minuteScrollRef.current) {
      const selectedElement = minuteScrollRef.current.children[selectedMinute] as HTMLElement
      if (selectedElement) {
        selectedElement.scrollIntoView({ block: "center", behavior: "smooth" })
      }
    }
  }, [selectedMinute])

  const isHourDisabled = (hour: number) => {
    // 如果有最小时间限制，小时小于最小值时禁用
    if (minTime) {
      const [minH] = minTime.split(":").map(Number)
      if (hour < minH) return true
    }
    // 如果有最大时间限制，小时大于最大值时禁用
    if (maxTime) {
      const [maxH] = maxTime.split(":").map(Number)
      if (hour > maxH) return true
    }
    return false
  }

  const isMinuteDisabled = (minute: number) => {
    // 如果最小和最大时间相同，禁用所有分钟（不允许相等）
    if (minTime && maxTime && minTime === maxTime) {
      return true
    }
    // 如果当前小时等于最小小时，分钟小于等于最小值时禁用
    if (minTime && selectedHour === minHour) {
      if (minute <= minMinute) return true
    }
    // 如果当前小时等于最大小时，分钟大于等于最大值时禁用
    if (maxTime && selectedHour === maxHour) {
      if (minute >= maxMinute) return true
    }
    return false
  }

  const handleHourChange = (hour: number) => {
    if (isHourDisabled(hour)) return
    
    let newMinute = parseInt(minutes, 10)
    
    // 如果选择了限制边界的小时，需要调整分钟
    if (minTime && hour === minHour) {
      const [minH, minM] = minTime.split(":").map(Number)
      if (newMinute <= minM) {
        newMinute = minM + 1
        // 如果调整后的分钟超出范围，使用该小时的最大可用分钟
        if (newMinute >= 60) {
          // 如果这个小时没有可用分钟，不允许选择
          return
        }
      }
    }
    if (maxTime && hour === maxHour) {
      const [maxH, maxM] = maxTime.split(":").map(Number)
      if (newMinute >= maxM) {
        newMinute = maxM - 1
        // 如果调整后的分钟超出范围，使用该小时的最小可用分钟
        if (newMinute < 0) {
          // 如果这个小时没有可用分钟，不允许选择
          return
        }
      }
    }
    
    const newTime = `${hour.toString().padStart(2, "0")}:${newMinute.toString().padStart(2, "0")}`
    onChange(newTime)
  }

  const handleMinuteChange = (minute: number) => {
    if (isMinuteDisabled(minute)) return
    const newTime = `${hours}:${minute.toString().padStart(2, "0")}`
    onChange(newTime)
  }

  return (
    <div className="flex flex-col items-center space-y-3">
      <label className="flex items-center gap-2 text-sm font-medium text-foreground">
        <Clock className="h-4 w-4" />
        时间
      </label>
      <div className="flex items-center gap-3">
        {/* 小时选择器 */}
        <div className="relative h-56 w-14 overflow-hidden">
          {/* 中间高亮指示器 */}
          <div className="absolute inset-x-0 top-1/2 z-10 h-12 -translate-y-1/2 border-y border-primary/20 bg-primary/5 pointer-events-none" />
          {/* 顶部渐变遮罩 */}
          <div className="absolute inset-x-0 top-0 z-20 h-16 bg-gradient-to-b from-background via-background/80 to-transparent pointer-events-none" />
          {/* 底部渐变遮罩 */}
          <div className="absolute inset-x-0 bottom-0 z-20 h-16 bg-gradient-to-t from-background via-background/80 to-transparent pointer-events-none" />
          <div
            ref={hourScrollRef}
            className="relative h-full flex flex-col overflow-y-auto scrollbar-hide py-20"
          >
            {hoursList.map((hour) => {
              const disabled = isHourDisabled(hour)
              const isSelected = selectedHour === hour && !disabled
              return (
                <button
                  key={hour}
                  type="button"
                  onClick={() => handleHourChange(hour)}
                  disabled={disabled}
                  className={cn(
                    "relative flex h-12 items-center justify-center text-sm font-medium transition-all duration-200",
                    "rounded-md",
                    disabled
                      ? "text-muted-foreground/40 cursor-not-allowed"
                      : "text-foreground cursor-pointer hover:bg-accent/50",
                    isSelected && "bg-primary text-primary-foreground shadow-md scale-105 z-10 mx-1"
                  )}
                >
                  {hour.toString().padStart(2, "0")}
                </button>
              )
            })}
          </div>
        </div>
        
        {/* 分隔符 */}
        <div className="text-lg font-semibold text-muted-foreground">:</div>
        
        {/* 分钟选择器 */}
        <div className="relative h-56 w-14 overflow-hidden">
          {/* 中间高亮指示器 */}
          <div className="absolute inset-x-0 top-1/2 z-10 h-12 -translate-y-1/2 border-y border-primary/20 bg-primary/5 pointer-events-none" />
          {/* 顶部渐变遮罩 */}
          <div className="absolute inset-x-0 top-0 z-20 h-16 bg-gradient-to-b from-background via-background/80 to-transparent pointer-events-none" />
          {/* 底部渐变遮罩 */}
          <div className="absolute inset-x-0 bottom-0 z-20 h-16 bg-gradient-to-t from-background via-background/80 to-transparent pointer-events-none" />
          <div
            ref={minuteScrollRef}
            className="relative h-full flex flex-col overflow-y-auto scrollbar-hide py-20"
          >
            {minutesList.map((minute) => {
              const disabled = isMinuteDisabled(minute)
              const isSelected = selectedMinute === minute && !disabled
              return (
                <button
                  key={minute}
                  type="button"
                  onClick={() => handleMinuteChange(minute)}
                  disabled={disabled}
                  className={cn(
                    "relative flex h-12 items-center justify-center text-sm font-medium transition-all duration-200",
                    "rounded-md",
                    disabled
                      ? "text-muted-foreground/40 cursor-not-allowed"
                      : "text-foreground cursor-pointer hover:bg-accent/50",
                    isSelected && "bg-primary text-primary-foreground shadow-md scale-105 z-10 mx-1"
                  )}
                >
                  {minute.toString().padStart(2, "0")}
                </button>
              )
            })}
          </div>
        </div>
      </div>
    </div>
  )
}

// 日期时间选择器组件
export function DateTimePicker({
  value,
  onChange,
  placeholder,
  minDate,
  maxDate,
  minTime,
  maxTime,
}: {
  value: string
  onChange: (value: string) => void
  placeholder?: string
  minDate?: Date
  maxDate?: Date
  minTime?: string
  maxTime?: string
}) {
  const [open, setOpen] = useState(false)
  const [selectedDate, setSelectedDate] = useState<Date | undefined>(
    value ? new Date(value) : undefined
  )
  const [timeValue, setTimeValue] = useState<string>(
    value ? format(new Date(value), "HH:mm") : ""
  )

  // 计算当前日期的时间限制
  const getCurrentTimeLimits = () => {
    if (!selectedDate) return { minTime: undefined, maxTime: undefined }
    
    const selectedDateStr = format(selectedDate, "yyyy-MM-dd")
    const minDateStr = minDate ? format(minDate, "yyyy-MM-dd") : null
    const maxDateStr = maxDate ? format(maxDate, "yyyy-MM-dd") : null
    
    let currentMinTime: string | undefined = undefined
    let currentMaxTime: string | undefined = undefined
    
    // 如果选中的日期等于最小日期，应用最小时间限制
    if (minDateStr && selectedDateStr === minDateStr) {
      currentMinTime = minTime
    }
    
    // 如果选中的日期等于最大日期，应用最大时间限制
    if (maxDateStr && selectedDateStr === maxDateStr) {
      currentMaxTime = maxTime
    }
    
    return { minTime: currentMinTime, maxTime: currentMaxTime }
  }

  const handleDateSelect = (date: Date | undefined) => {
    if (date) {
      setSelectedDate(date)
      // 如果已选择时间，合并日期和时间
      if (timeValue) {
        const [hours, minutes] = timeValue.split(":")
        const newDate = new Date(date)
        newDate.setHours(parseInt(hours, 10), parseInt(minutes, 10), 0, 0)
        
        // 检查是否违反限制
        if (minDate && newDate <= minDate) {
          // 如果违反最小限制，设置为最小时间之后
          const minDatePlusOne = new Date(minDate)
          minDatePlusOne.setMinutes(minDatePlusOne.getMinutes() + 1)
          onChange(minDatePlusOne.toISOString())
          setTimeValue(format(minDatePlusOne, "HH:mm"))
          return
        }
        if (maxDate && newDate >= maxDate) {
          // 如果违反最大限制，设置为最大时间之前
          const maxDateMinusOne = new Date(maxDate)
          maxDateMinusOne.setMinutes(maxDateMinusOne.getMinutes() - 1)
          onChange(maxDateMinusOne.toISOString())
          setTimeValue(format(maxDateMinusOne, "HH:mm"))
          return
        }
        
        onChange(newDate.toISOString())
      } else {
        // 只选择日期，时间设为当前时间
        const newDate = new Date(date)
        const now = new Date()
        newDate.setHours(now.getHours(), now.getMinutes(), 0, 0)
        
        // 检查限制
        if (minDate && newDate <= minDate) {
          const minDatePlusOne = new Date(minDate)
          minDatePlusOne.setMinutes(minDatePlusOne.getMinutes() + 1)
          onChange(minDatePlusOne.toISOString())
          setTimeValue(format(minDatePlusOne, "HH:mm"))
        } else if (maxDate && newDate >= maxDate) {
          const maxDateMinusOne = new Date(maxDate)
          maxDateMinusOne.setMinutes(maxDateMinusOne.getMinutes() - 1)
          onChange(maxDateMinusOne.toISOString())
          setTimeValue(format(maxDateMinusOne, "HH:mm"))
        } else {
        onChange(newDate.toISOString())
        setTimeValue(format(newDate, "HH:mm"))
        }
      }
    }
  }

  const handleTimeChange = (time: string) => {
    setTimeValue(time)
    if (selectedDate && time) {
      const [hours, minutes] = time.split(":")
      const newDate = new Date(selectedDate)
      newDate.setHours(parseInt(hours, 10), parseInt(minutes, 10), 0, 0)
      
      // 检查是否违反限制
      if (minDate && newDate <= minDate) {
        return // 不更新，保持原值
      }
      if (maxDate && newDate >= maxDate) {
        return // 不更新，保持原值
      }
      
      onChange(newDate.toISOString())
    }
  }

  const timeLimits = getCurrentTimeLimits()

  // 同步外部值变化
  useEffect(() => {
    if (value) {
      const date = new Date(value)
      setSelectedDate(date)
      setTimeValue(format(date, "HH:mm"))
    }
  }, [value])

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          variant="outline"
          className="flex-1 justify-start text-left font-normal"
        >
          <CalendarIcon className="mr-2 h-4 w-4" />
          {value ? (
            format(new Date(value), "yyyy年MM月dd日 HH:mm", { locale: zhCN })
          ) : (
            <span className="text-muted-foreground">{placeholder || "选择日期时间"}</span>
          )}
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-auto p-0" align="start">
        <div className="flex p-3 gap-4 rounded-md overflow-hidden">
          <div className="rounded-md overflow-hidden">
          <Calendar
              locale={zhCN}
            mode="single"
            selected={selectedDate}
            onSelect={handleDateSelect}
            initialFocus
              disabled={(date) => {
                if (minDate) {
                  const minDateOnly = new Date(minDate.getFullYear(), minDate.getMonth(), minDate.getDate())
                  const dateOnly = new Date(date.getFullYear(), date.getMonth(), date.getDate())
                  if (dateOnly < minDateOnly) return true
                }
                if (maxDate) {
                  const maxDateOnly = new Date(maxDate.getFullYear(), maxDate.getMonth(), maxDate.getDate())
                  const dateOnly = new Date(date.getFullYear(), date.getMonth(), date.getDate())
                  if (dateOnly > maxDateOnly) return true
                }
                return false
              }}
            />
          </div>
          <div className="border-l pl-4 flex flex-col justify-center">
            <TimePicker 
                value={timeValue}
                onChange={handleTimeChange}
              minTime={timeLimits.minTime}
              maxTime={timeLimits.maxTime}
              />
          </div>
        </div>
      </PopoverContent>
    </Popover>
  )
}

