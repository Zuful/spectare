import WatchClient from './WatchClient'

export function generateStaticParams() {
  return Array.from({ length: 20 }, (_, i) => ({ id: String(i + 1) }))
}

export default async function WatchPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params
  return <WatchClient id={id} />
}
