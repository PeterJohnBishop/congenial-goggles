# congenial-goggles

          ┌───────────────────────────┐
          │        Clients            │
          └────────────┬──────────────┘
                       │
                 HTTP / WebSocket
                       │
          ┌───────────────────────────┐
          │   API Orchestrator Server │
          │ (e.g., Go + Gin / Fiber)  │
          │ - Tracks container states │
          │ - Routes requests         │
          │ - Handles concurrency     │
          └────────────┬──────────────┘
                       │
         ┌─────────────┼────────────────┐
         │              │                │
 ┌────────────┐  ┌────────────┐   ┌────────────┐
 │ Container 1│  │ Container 2│   │ Container 3│
 │  (Service) │  │  (Service) │   │  (Service) │
 └────────────┘  └────────────┘   └────────────┘
