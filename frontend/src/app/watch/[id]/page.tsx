import WatchClient from './WatchClient'

export function generateStaticParams() {
  return Array.from({ length: 20 }, (_, i) => ({ id: String(i + 1) }))
}

export default function WatchPage({ params }: { params: { id: string } }) {
  return <WatchClient id={params.id} />
}
