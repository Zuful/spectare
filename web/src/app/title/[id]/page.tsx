import TitlePageClient from './TitlePageClient'

export function generateStaticParams() {
  return Array.from({ length: 20 }, (_, i) => ({ id: String(i + 1) }))
}

export default async function TitlePage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params
  return <TitlePageClient id={id} />
}
