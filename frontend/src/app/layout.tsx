import type { Metadata } from 'next'
import { Inter, Manrope } from 'next/font/google'
import './globals.css'

const inter = Inter({ subsets: ['latin'], variable: '--font-inter' })
const manrope = Manrope({ subsets: ['latin'], variable: '--font-manrope' })

export const metadata: Metadata = {
  title: 'Spectare',
  description: 'Premium VOD streaming platform',
}

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className={`${inter.variable} ${manrope.variable} dark h-full`}>
      <body className="min-h-full bg-[#131313] text-[#e5e2e1] antialiased">
        {children}
      </body>
    </html>
  )
}
