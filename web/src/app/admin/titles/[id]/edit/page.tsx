import EditTitleClient from './EditTitleClient'

export function generateStaticParams() {
  return Array.from({ length: 20 }, (_, i) => ({ id: String(i + 1) }))
}

export default async function EditTitlePage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params
  return <EditTitleClient id={id} />
}
