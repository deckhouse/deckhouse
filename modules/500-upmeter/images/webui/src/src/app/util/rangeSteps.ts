export interface RangeStep {
  step: number
}

export const RangeStepsLadder: RangeStep[] = [
  { step: 60 * 5 }, // 5min, Last 1h
  { step: 60 * 15 }, // 15m  Last 3h
  { step: 60 * 30 }, // 30m Last 6h
  { step: 60 * 60 }, // 1h Last 12h
  { step: 60 * 60 * 2 }, // 2h Last 24h
  { step: 60 * 60 * 4 }, // 4h Last 2d
  { step: 60 * 60 * 14 }, // 14h Last 7d
  { step: 60 * 60 * 24 + 60 * 60 * 12 }, // 1d 12h Last 30d
  { step: 60 * 60 * 24 * 7 + 60 * 60 * 12 }, // 7d 12h Last 90d
  { step: 60 * 60 * 24 * 15 + 60 * 60 * 8 }, // 15d 8h Last 6m
  { step: 60 * 60 * 24 * 30 + 60 * 60 * 12 }, // 1mon 12h Last 1y
  { step: 60 * 60 * 24 * 30 * 2 + 60 * 60 * 22 }, // 2mon 22h Last 2y
  { step: 60 * 60 * 24 * 30 * 5 + 60 * 60 * 24 * 2 + 60 * 60 * 6 }, // 5mon 2day 6h  Last 5y
]
