#!/usr/bin/env bash
set -euo pipefail

PORT="${SPECTRA_PORT:-8090}"
BASE="http://localhost:${PORT}"

echo "Ingesting sample AI trace data into Spectra at ${BASE}..."

# ── Trace 1: Simple LLM conversation ────────────────────────────────
curl -sf -X POST "${BASE}/v1/spans" \
  -H "Content-Type: application/json" \
  -d '{
  "spans": [
    {
      "span_id": "conv-1-root",
      "trace_id": "trace-conv-001",
      "name": "chat_completion",
      "kind": "chain",
      "status": "ok",
      "start_time": "2024-06-15T10:00:00Z",
      "end_time": "2024-06-15T10:00:03.200Z",
      "input": "Explain quantum computing in simple terms",
      "output": "Quantum computing uses qubits that can be 0, 1, or both at once...",
      "metrics": {"latency_ms": 3200, "prompt_tokens": 12, "completion_tokens": 85, "total_tokens": 97, "cost_usd": 0.0029}
    },
    {
      "span_id": "conv-1-llm",
      "trace_id": "trace-conv-001",
      "parent_id": "conv-1-root",
      "name": "gpt4o_call",
      "kind": "llm",
      "status": "ok",
      "start_time": "2024-06-15T10:00:00.100Z",
      "end_time": "2024-06-15T10:00:03.100Z",
      "input": "System: You are a helpful assistant.\nUser: Explain quantum computing in simple terms",
      "output": "Quantum computing uses qubits that can be 0, 1, or both at once thanks to superposition. This lets quantum computers explore many solutions simultaneously, making them powerful for specific problems like cryptography and drug discovery.",
      "metadata": {"model": "gpt-4o", "temperature": "0.7"},
      "metrics": {"latency_ms": 3000, "prompt_tokens": 12, "completion_tokens": 85, "total_tokens": 97, "cost_usd": 0.0029}
    }
  ]
}' > /dev/null
echo "  [1/8] Chat conversation trace"

# ── Trace 2: RAG pipeline ───────────────────────────────────────────
curl -sf -X POST "${BASE}/v1/spans" \
  -H "Content-Type: application/json" \
  -d '{
  "spans": [
    {
      "span_id": "rag-2-root",
      "trace_id": "trace-rag-002",
      "name": "rag_query",
      "kind": "chain",
      "status": "ok",
      "start_time": "2024-06-15T10:05:00Z",
      "end_time": "2024-06-15T10:05:04.500Z",
      "input": "What is our refund policy for enterprise customers?",
      "output": "Enterprise customers are eligible for a full refund within 90 days of purchase...",
      "metrics": {"latency_ms": 4500, "total_tokens": 340, "cost_usd": 0.0051}
    },
    {
      "span_id": "rag-2-embed",
      "trace_id": "trace-rag-002",
      "parent_id": "rag-2-root",
      "name": "embed_query",
      "kind": "llm",
      "status": "ok",
      "start_time": "2024-06-15T10:05:00.050Z",
      "end_time": "2024-06-15T10:05:00.250Z",
      "input": "What is our refund policy for enterprise customers?",
      "metadata": {"model": "text-embedding-3-small"},
      "metrics": {"latency_ms": 200, "prompt_tokens": 10, "total_tokens": 10, "cost_usd": 0.0001}
    },
    {
      "span_id": "rag-2-retrieve",
      "trace_id": "trace-rag-002",
      "parent_id": "rag-2-root",
      "name": "vector_search",
      "kind": "retriever",
      "status": "ok",
      "start_time": "2024-06-15T10:05:00.300Z",
      "end_time": "2024-06-15T10:05:01.100Z",
      "input": "query_embedding: [0.012, -0.034, ...]",
      "output": "Retrieved 5 documents from knowledge base",
      "metadata": {"index": "enterprise-docs", "top_k": "5"},
      "metrics": {"latency_ms": 800}
    },
    {
      "span_id": "rag-2-llm",
      "trace_id": "trace-rag-002",
      "parent_id": "rag-2-root",
      "name": "claude_sonnet_call",
      "kind": "llm",
      "status": "ok",
      "start_time": "2024-06-15T10:05:01.200Z",
      "end_time": "2024-06-15T10:05:04.400Z",
      "input": "Based on the following context, answer the question...\n\nContext: [5 retrieved documents]\n\nQuestion: What is our refund policy for enterprise customers?",
      "output": "Enterprise customers are eligible for a full refund within 90 days of purchase. After 90 days, a prorated refund is available. Contact your account manager to initiate the process.",
      "metadata": {"model": "claude-3.5-sonnet", "temperature": "0.3"},
      "metrics": {"latency_ms": 3200, "prompt_tokens": 250, "completion_tokens": 80, "total_tokens": 330, "cost_usd": 0.005}
    }
  ]
}' > /dev/null
echo "  [2/8] RAG pipeline trace"

# ── Trace 3: Agent with tool calls ──────────────────────────────────
curl -sf -X POST "${BASE}/v1/spans" \
  -H "Content-Type: application/json" \
  -d '{
  "spans": [
    {
      "span_id": "agent-3-root",
      "trace_id": "trace-agent-003",
      "name": "booking_agent",
      "kind": "agent",
      "status": "ok",
      "start_time": "2024-06-15T10:10:00Z",
      "end_time": "2024-06-15T10:10:08Z",
      "input": "Book a flight from SFO to JFK for next Friday",
      "output": "I found 3 flights. The best option is UA 234 departing at 8:00 AM, arriving at 4:30 PM. Shall I book it?",
      "metrics": {"latency_ms": 8000, "total_tokens": 520, "cost_usd": 0.012}
    },
    {
      "span_id": "agent-3-plan",
      "trace_id": "trace-agent-003",
      "parent_id": "agent-3-root",
      "name": "gpt4o_plan",
      "kind": "llm",
      "status": "ok",
      "start_time": "2024-06-15T10:10:00.100Z",
      "end_time": "2024-06-15T10:10:01.500Z",
      "input": "User wants to book a flight. I should search for available flights first.",
      "output": "I will use the flight_search tool to find flights from SFO to JFK.",
      "metadata": {"model": "gpt-4o", "step": "planning"},
      "metrics": {"latency_ms": 1400, "prompt_tokens": 80, "completion_tokens": 40, "total_tokens": 120, "cost_usd": 0.003}
    },
    {
      "span_id": "agent-3-tool",
      "trace_id": "trace-agent-003",
      "parent_id": "agent-3-root",
      "name": "flight_search",
      "kind": "tool",
      "status": "ok",
      "start_time": "2024-06-15T10:10:01.600Z",
      "end_time": "2024-06-15T10:10:04.600Z",
      "input": "{\"origin\": \"SFO\", \"destination\": \"JFK\", \"date\": \"2024-06-21\"}",
      "output": "[{\"flight\": \"UA 234\", \"depart\": \"08:00\", \"arrive\": \"16:30\", \"price\": 389}, {\"flight\": \"AA 100\", \"depart\": \"10:15\", \"arrive\": \"18:45\", \"price\": 425}, {\"flight\": \"DL 512\", \"depart\": \"14:00\", \"arrive\": \"22:20\", \"price\": 352}]",
      "metadata": {"api": "amadeus", "endpoint": "/v2/shopping/flight-offers"},
      "metrics": {"latency_ms": 3000}
    },
    {
      "span_id": "agent-3-respond",
      "trace_id": "trace-agent-003",
      "parent_id": "agent-3-root",
      "name": "gpt4o_respond",
      "kind": "llm",
      "status": "ok",
      "start_time": "2024-06-15T10:10:04.700Z",
      "end_time": "2024-06-15T10:10:07.900Z",
      "input": "Flight search results: [3 flights found]. Summarize options for user.",
      "output": "I found 3 flights. The best option is UA 234 departing at 8:00 AM, arriving at 4:30 PM for $389. Shall I book it?",
      "metadata": {"model": "gpt-4o", "step": "responding"},
      "metrics": {"latency_ms": 3200, "prompt_tokens": 200, "completion_tokens": 60, "total_tokens": 260, "cost_usd": 0.006}
    }
  ]
}' > /dev/null
echo "  [3/8] Agent with tool calls trace"

# ── Trace 4: Failed LLM call ────────────────────────────────────────
curl -sf -X POST "${BASE}/v1/spans" \
  -H "Content-Type: application/json" \
  -d '{
  "spans": [
    {
      "span_id": "err-4-root",
      "trace_id": "trace-error-004",
      "name": "summarize_document",
      "kind": "chain",
      "status": "error",
      "start_time": "2024-06-15T10:15:00Z",
      "end_time": "2024-06-15T10:15:30.500Z",
      "input": "Summarize the attached 200-page contract",
      "output": "Error: context length exceeded",
      "metrics": {"latency_ms": 30500}
    },
    {
      "span_id": "err-4-llm",
      "trace_id": "trace-error-004",
      "parent_id": "err-4-root",
      "name": "gpt4_call",
      "kind": "llm",
      "status": "error",
      "start_time": "2024-06-15T10:15:00.100Z",
      "end_time": "2024-06-15T10:15:30.400Z",
      "input": "[200 pages of contract text...]",
      "output": "Error 400: This model maximum context length is 128000 tokens. However, your messages resulted in 195000 tokens.",
      "metadata": {"model": "gpt-4-turbo", "error_code": "context_length_exceeded"},
      "events": [{"name": "error", "timestamp": "2024-06-15T10:15:30.400Z", "data": {"type": "context_length_exceeded", "max_tokens": 128000, "requested_tokens": 195000}}],
      "metrics": {"latency_ms": 30300, "prompt_tokens": 195000, "cost_usd": 0.0}
    }
  ]
}' > /dev/null
echo "  [4/8] Error trace (context length exceeded)"

# ── Trace 5: Multi-step code generation ──────────────────────────────
curl -sf -X POST "${BASE}/v1/spans" \
  -H "Content-Type: application/json" \
  -d '{
  "spans": [
    {
      "span_id": "code-5-root",
      "trace_id": "trace-codegen-005",
      "name": "code_generation",
      "kind": "chain",
      "status": "ok",
      "start_time": "2024-06-15T10:20:00Z",
      "end_time": "2024-06-15T10:20:06Z",
      "input": "Write a Python function to calculate fibonacci numbers with memoization",
      "output": "def fibonacci(n, memo={}):\n    if n in memo: return memo[n]\n    if n <= 1: return n\n    memo[n] = fibonacci(n-1, memo) + fibonacci(n-2, memo)\n    return memo[n]",
      "metrics": {"latency_ms": 6000, "total_tokens": 280, "cost_usd": 0.0042}
    },
    {
      "span_id": "code-5-generate",
      "trace_id": "trace-codegen-005",
      "parent_id": "code-5-root",
      "name": "claude_opus_generate",
      "kind": "llm",
      "status": "ok",
      "start_time": "2024-06-15T10:20:00.100Z",
      "end_time": "2024-06-15T10:20:03.500Z",
      "input": "Write a Python function to calculate fibonacci numbers with memoization. Include type hints and docstring.",
      "output": "def fibonacci(n: int, memo: dict[int, int] = {}) -> int:\n    \"\"\"Calculate nth Fibonacci number with memoization.\"\"\"\n    if n in memo: return memo[n]\n    if n <= 1: return n\n    memo[n] = fibonacci(n-1, memo) + fibonacci(n-2, memo)\n    return memo[n]",
      "metadata": {"model": "claude-opus-4", "temperature": "0"},
      "metrics": {"latency_ms": 3400, "prompt_tokens": 30, "completion_tokens": 120, "total_tokens": 150, "cost_usd": 0.003}
    },
    {
      "span_id": "code-5-review",
      "trace_id": "trace-codegen-005",
      "parent_id": "code-5-root",
      "name": "claude_sonnet_review",
      "kind": "llm",
      "status": "ok",
      "start_time": "2024-06-15T10:20:03.600Z",
      "end_time": "2024-06-15T10:20:05.900Z",
      "input": "Review this code for correctness and suggest improvements:\ndef fibonacci(n, memo={})...",
      "output": "The code is correct. Suggestion: use functools.lru_cache instead of mutable default argument to avoid the mutable default anti-pattern.",
      "metadata": {"model": "claude-3.5-sonnet", "temperature": "0.2"},
      "metrics": {"latency_ms": 2300, "prompt_tokens": 80, "completion_tokens": 50, "total_tokens": 130, "cost_usd": 0.0012}
    }
  ]
}' > /dev/null
echo "  [5/8] Code generation + review trace"

# ── Trace 6: Batch of tool spans ─────────────────────────────────────
curl -sf -X POST "${BASE}/v1/spans" \
  -H "Content-Type: application/json" \
  -d '{
  "spans": [
    {
      "span_id": "tool-6-web",
      "trace_id": "trace-tools-006",
      "name": "web_search",
      "kind": "tool",
      "status": "ok",
      "start_time": "2024-06-15T10:25:00Z",
      "end_time": "2024-06-15T10:25:02Z",
      "input": "{\"query\": \"latest AI safety research 2024\"}",
      "output": "{\"results\": [{\"title\": \"Anthropic Publishes RSP\", \"url\": \"...\"}, {\"title\": \"DeepMind Safety Report\", \"url\": \"...\"}]}",
      "metadata": {"api": "serper", "result_count": "10"},
      "metrics": {"latency_ms": 2000}
    },
    {
      "span_id": "tool-6-calc",
      "trace_id": "trace-tools-006",
      "name": "calculator",
      "kind": "tool",
      "status": "ok",
      "start_time": "2024-06-15T10:25:02.100Z",
      "end_time": "2024-06-15T10:25:02.150Z",
      "input": "{\"expression\": \"(1500 * 0.15) + (2300 * 0.22)\"}",
      "output": "{\"result\": 731.0}",
      "metrics": {"latency_ms": 50}
    },
    {
      "span_id": "tool-6-db",
      "trace_id": "trace-tools-006",
      "name": "sql_query",
      "kind": "tool",
      "status": "error",
      "start_time": "2024-06-15T10:25:02.200Z",
      "end_time": "2024-06-15T10:25:05.200Z",
      "input": "{\"query\": \"SELECT * FROM users WHERE email LIKE '%@example.com'\"}",
      "output": "Error: query timeout after 3000ms",
      "metadata": {"database": "analytics-prod", "timeout_ms": "3000"},
      "events": [{"name": "timeout", "timestamp": "2024-06-15T10:25:05.200Z", "data": {"reason": "query_timeout"}}],
      "metrics": {"latency_ms": 3000}
    }
  ]
}' > /dev/null
echo "  [6/8] Tool calls trace (search, calc, SQL error)"

# ── Trace 7: Retriever comparison ────────────────────────────────────
curl -sf -X POST "${BASE}/v1/spans" \
  -H "Content-Type: application/json" \
  -d '{
  "spans": [
    {
      "span_id": "ret-7-root",
      "trace_id": "trace-retriever-007",
      "name": "hybrid_search",
      "kind": "chain",
      "status": "ok",
      "start_time": "2024-06-15T10:30:00Z",
      "end_time": "2024-06-15T10:30:03Z",
      "input": "How to configure Kubernetes network policies?",
      "metrics": {"latency_ms": 3000}
    },
    {
      "span_id": "ret-7-vector",
      "trace_id": "trace-retriever-007",
      "parent_id": "ret-7-root",
      "name": "pinecone_search",
      "kind": "retriever",
      "status": "ok",
      "start_time": "2024-06-15T10:30:00.100Z",
      "end_time": "2024-06-15T10:30:01.300Z",
      "input": "kubernetes network policies configuration",
      "output": "Found 8 relevant documents (scores: 0.92, 0.89, 0.87, 0.85, 0.82, 0.78, 0.75, 0.71)",
      "metadata": {"index": "k8s-docs", "namespace": "networking", "top_k": "8"},
      "metrics": {"latency_ms": 1200}
    },
    {
      "span_id": "ret-7-keyword",
      "trace_id": "trace-retriever-007",
      "parent_id": "ret-7-root",
      "name": "elasticsearch_search",
      "kind": "retriever",
      "status": "ok",
      "start_time": "2024-06-15T10:30:00.100Z",
      "end_time": "2024-06-15T10:30:00.900Z",
      "input": "kubernetes network policies",
      "output": "Found 12 documents matching keywords",
      "metadata": {"index": "k8s-docs-bm25", "top_k": "12"},
      "metrics": {"latency_ms": 800}
    }
  ]
}' > /dev/null
echo "  [7/8] Hybrid retriever trace"

# ── Trace 8: Complex agent loop ──────────────────────────────────────
curl -sf -X POST "${BASE}/v1/spans" \
  -H "Content-Type: application/json" \
  -d '{
  "spans": [
    {
      "span_id": "loop-8-root",
      "trace_id": "trace-agent-loop-008",
      "name": "research_agent",
      "kind": "agent",
      "status": "ok",
      "start_time": "2024-06-15T10:35:00Z",
      "end_time": "2024-06-15T10:35:15Z",
      "input": "Research and compare the top 3 vector databases for production use",
      "output": "Based on my research, here is a comparison of Pinecone, Weaviate, and Milvus...",
      "metrics": {"latency_ms": 15000, "total_tokens": 1800, "cost_usd": 0.045}
    },
    {
      "span_id": "loop-8-think1",
      "trace_id": "trace-agent-loop-008",
      "parent_id": "loop-8-root",
      "name": "claude_think_step1",
      "kind": "llm",
      "status": "ok",
      "start_time": "2024-06-15T10:35:00.100Z",
      "end_time": "2024-06-15T10:35:02Z",
      "input": "I need to research vector databases. Let me search for each one.",
      "output": "I will search for Pinecone, Weaviate, and Milvus to compare features, pricing, and performance.",
      "metadata": {"model": "claude-opus-4", "step": "1"},
      "metrics": {"latency_ms": 1900, "prompt_tokens": 50, "completion_tokens": 40, "total_tokens": 90, "cost_usd": 0.003}
    },
    {
      "span_id": "loop-8-search1",
      "trace_id": "trace-agent-loop-008",
      "parent_id": "loop-8-root",
      "name": "web_search_pinecone",
      "kind": "tool",
      "status": "ok",
      "start_time": "2024-06-15T10:35:02.100Z",
      "end_time": "2024-06-15T10:35:04Z",
      "input": "{\"query\": \"Pinecone vector database review 2024 pricing performance\"}",
      "output": "{\"results\": 8}",
      "metrics": {"latency_ms": 1900}
    },
    {
      "span_id": "loop-8-search2",
      "trace_id": "trace-agent-loop-008",
      "parent_id": "loop-8-root",
      "name": "web_search_weaviate",
      "kind": "tool",
      "status": "ok",
      "start_time": "2024-06-15T10:35:04.100Z",
      "end_time": "2024-06-15T10:35:06Z",
      "input": "{\"query\": \"Weaviate vector database review 2024\"}",
      "output": "{\"results\": 6}",
      "metrics": {"latency_ms": 1900}
    },
    {
      "span_id": "loop-8-search3",
      "trace_id": "trace-agent-loop-008",
      "parent_id": "loop-8-root",
      "name": "web_search_milvus",
      "kind": "tool",
      "status": "ok",
      "start_time": "2024-06-15T10:35:06.100Z",
      "end_time": "2024-06-15T10:35:08Z",
      "input": "{\"query\": \"Milvus vector database review 2024\"}",
      "output": "{\"results\": 7}",
      "metrics": {"latency_ms": 1900}
    },
    {
      "span_id": "loop-8-synthesize",
      "trace_id": "trace-agent-loop-008",
      "parent_id": "loop-8-root",
      "name": "claude_synthesize",
      "kind": "llm",
      "status": "ok",
      "start_time": "2024-06-15T10:35:08.100Z",
      "end_time": "2024-06-15T10:35:14.900Z",
      "input": "Based on search results for Pinecone, Weaviate, and Milvus, create a detailed comparison...",
      "output": "## Vector Database Comparison\n\n### Pinecone\n- Fully managed, serverless option\n- Best for: teams wanting zero ops overhead\n- Pricing: starts at $70/mo\n\n### Weaviate\n- Open source, hybrid search built-in\n- Best for: teams wanting flexibility\n- Pricing: free self-hosted, cloud from $25/mo\n\n### Milvus\n- Open source, highest raw performance\n- Best for: large-scale deployments\n- Pricing: free self-hosted",
      "metadata": {"model": "claude-opus-4", "step": "final"},
      "metrics": {"latency_ms": 6800, "prompt_tokens": 800, "completion_tokens": 400, "total_tokens": 1200, "cost_usd": 0.036}
    }
  ]
}' > /dev/null
echo "  [8/8] Multi-step research agent trace"

echo ""
echo "Done! Ingested 8 traces with 25 spans."
echo ""
echo "Try these queries:"
echo "  curl -s http://localhost:${PORT}/v1/traces/trace-rag-002 | jq"
echo "  curl -s -X POST http://localhost:${PORT}/v1/search -d '{\"query\": \"kubernetes\", \"limit\": 10}' | jq"
echo "  curl -s -X POST http://localhost:${PORT}/v1/query -d '{\"filters\": [{\"field\": \"kind\", \"operator\": \"eq\", \"value\": \"llm\"}], \"limit\": 20}' | jq"
echo "  curl -s -X POST http://localhost:${PORT}/v1/query -d '{\"filters\": [{\"field\": \"status\", \"operator\": \"eq\", \"value\": \"error\"}], \"limit\": 10}' | jq"
