# Developer Onboarding

Welcome to the Fixora project! This guide will help you set up your local development environment and understand the project architecture.

## Tech Stack
- **Backend:** Go (Golang)
- **Frontend:** React + TypeScript + Vite
- **Database:** PostgreSQL
- **Infrastructure:** Kubernetes, Helm
- **Integrations:** Slack, Google Chat, GitHub/GitLab APIs

## Local Setup

### Prerequisites
- Go 1.22+
- Node.js & npm (for UI)
- Docker & Kubernetes (Kind or Minikube recommended)
- PostgreSQL instance

### Running the Backend locally
1. Clone the repository:
   ```bash
   git clone https://github.com/baka126/fixora.git
   cd fixora
   ```
2. Install dependencies:
   ```bash
   go mod download
   ```
3. Set up environment variables (copy from `charts/fixora/values.yaml` logic):
   ```bash
   export DB_HOST=localhost
   export DB_PORT=5432
   export DB_USER=postgres
   export DB_NAME=fixora
   export AI_PROVIDER=gemini
   export AI_API_KEY=your-key
   ```
4. Run the application:
   ```bash
   go run cmd/fixora/main.go
   ```

### Running the UI locally
1. Navigate to the `ui/` directory:
   ```bash
   cd ui
   ```
2. Install dependencies:
   ```bash
   npm install
   ```
3. Run the development server:
   ```bash
   npm run dev
   ```

## Project Structure

- `cmd/fixora/`: Application entry point.
- `pkg/`: Core logic packages.
  - `ai/`: AI provider integrations (Gemini, OpenAI, Anthropic).
  - `analyzer/`: Kubernetes resource analysis logic.
  - `controller/`: Orchestration and business logic.
  - `db/`: Database interactions.
  - `server/`: HTTP server and API handlers.
  - `notifications/`: Slack and Google Chat integrations.
- `charts/fixora/`: Helm chart for deployment.
- `ui/`: React-based dashboard source code.
- `docs/`: Documentation site (Docsify).

## Contribution Workflow
1. Create a feature branch.
2. Implement your changes with tests.
3. Ensure `go test ./...` passes.
4. Open a Pull Request.

## Testing
Fixora uses standard Go testing:
```bash
go test -v ./pkg/...
```
For UI tests:
```bash
cd ui
npm test
```
