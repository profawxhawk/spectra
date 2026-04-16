import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { TooltipProvider } from '@/components/ui/tooltip'
import DashboardPage from '@/pages/dashboard'
import ExplorerPage from '@/pages/explorer'
import SearchPage from '@/pages/search'
import TraceDetailPage from '@/pages/trace-detail'
import LiveTailPage from '@/pages/live-tail'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 10_000,
      retry: 2,
    },
  },
})

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <TooltipProvider>
        <BrowserRouter>
          <Routes>
            <Route path="/" element={<DashboardPage />} />
            <Route path="/explorer" element={<ExplorerPage />} />
            <Route path="/search" element={<SearchPage />} />
            <Route path="/trace/:traceId" element={<TraceDetailPage />} />
            <Route path="/live" element={<LiveTailPage />} />
          </Routes>
        </BrowserRouter>
      </TooltipProvider>
    </QueryClientProvider>
  )
}
