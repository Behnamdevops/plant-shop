import type { CSSProperties } from 'react'
export type IconName =
  | 'leaf'
  | 'search'
  | 'bag'
  | 'heart'
  | 'user'
  | 'menu'
  | 'arrow'
  | 'sun'
  | 'drop'
  | 'shield'
  | 'truck'
  | 'close'
  | 'grid'
  | 'plus'
const paths: Record<IconName, string[]> = {
  leaf: ['M20 4c-9-1-16 3-16 9a7 7 0 0 0 7 7c6 0 10-7 9-16Z', 'M4 20 15 9'],
  search: ['M21 21l-5-5', 'M18 10a8 8 0 1 1-16 0 8 8 0 0 1 16 0'],
  bag: ['M5 7h14l1 14H4L5 7Z', 'M8 8V6a4 4 0 0 1 8 0v2'],
  heart: [
    'M20.8 4.6a5.5 5.5 0 0 0-7.8 0L12 5.7l-1.1-1.1a5.5 5.5 0 0 0-7.8 7.8L12 21l8.8-8.6a5.5 5.5 0 0 0 0-7.8Z',
  ],
  user: [
    'M16 7a4 4 0 1 1-8 0 4 4 0 0 1 8 0',
    'M3 21v-2a7 7 0 0 1 7-7h4a7 7 0 0 1 7 7v2',
  ],
  menu: ['M3 6h18M3 12h18M3 18h18'],
  arrow: ['M20 12H4m6-6-6 6 6 6'],
  sun: [
    'M12 2v2m0 16v2M2 12h2m16 0h2M5 5l1 1m12 12 1 1M5 19l1-1M18 6l1-1',
    'M17 12a5 5 0 1 1-10 0 5 5 0 0 1 10 0',
  ],
  drop: ['M12 2S4 11 4 16a8 8 0 0 0 16 0c0-5-8-14-8-14Z'],
  shield: ['M12 2 3 6v6c0 5 9 10 9 10s9-5 9-10V6l-9-4Z', 'm8 12 3 3 5-6'],
  truck: [
    'M1 5h13v12H1V5Zm13 5h4l4 4v3h-8',
    'M8 18a3 3 0 1 1-6 0 3 3 0 0 1 6 0M22 18a3 3 0 1 1-6 0 3 3 0 0 1 6 0',
  ],
  close: ['m6 6 12 12M18 6 6 18'],
  grid: ['M3 3h7v7H3V3Zm11 0h7v7h-7V3ZM3 14h7v7H3v-7Zm11 0h7v7h-7v-7Z'],
  plus: ['M12 4v16M4 12h16'],
}
export default function Icon({
  name,
  size = 22,
  style,
}: {
  name: IconName
  size?: number
  style?: CSSProperties
}) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.65"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      style={style}
    >
      {paths[name].map((d, i) => (
        <path key={i} d={d} />
      ))}
    </svg>
  )
}
