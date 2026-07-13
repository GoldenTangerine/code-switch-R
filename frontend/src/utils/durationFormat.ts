/**
 * @name: 时长格式化工具
 * @Descripttion: 将秒数统一格式化为自适应时长单位
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-13 20:56:42
 * @LastEditTime: 2026-07-13 20:56:42
 * @FilePath: frontend/src/utils/durationFormat.ts
 */

const EMPTY_DURATION = '—'

function pad2(value: number) {
  return String(value).padStart(2, '0')
}

export function formatAdaptiveDurationSeconds(value: unknown) {
  const seconds = Number(value)
  if (!Number.isFinite(seconds) || seconds <= 0) return EMPTY_DURATION

  const totalMilliseconds = Math.max(1, Math.round(seconds * 1000))
  if (totalMilliseconds < 1000) {
    return `${totalMilliseconds}ms`
  }

  const normalizedSeconds = totalMilliseconds / 1000
  if (normalizedSeconds < 60) {
    const precisionScale = totalMilliseconds < 10_000 ? 100 : 10
    const millisecondsPerPrecisionUnit = 1000 / precisionScale
    const roundedSeconds = Math.round(totalMilliseconds / millisecondsPerPrecisionUnit) / precisionScale
    if (roundedSeconds < 60) {
      return `${roundedSeconds}s`
    }
  }

  const totalSeconds = Math.round(normalizedSeconds)
  const remainingSeconds = totalSeconds % 60
  const totalMinutes = Math.floor(totalSeconds / 60)
  if (totalMinutes < 60) {
    return `${totalMinutes}m ${pad2(remainingSeconds)}s`
  }

  const hours = Math.floor(totalMinutes / 60)
  const remainingMinutes = totalMinutes % 60
  return `${hours}h ${pad2(remainingMinutes)}m ${pad2(remainingSeconds)}s`
}
