# Oba Web Admin Panel

Web-based administration panel for Oba LDAP Server built with React, Vite, and Tailwind CSS.

## Features

- Real-time dashboard with server statistics (auto-refresh every 5 seconds)
- LDAP entry browser and search
- User and group management with lock/unlock support
- ACL rule editor
- Configuration management
- Log viewer with filtering, export, and Node ID display (cluster mode)
- Password change functionality

## Development

```bash
# Install dependencies
npm install

# Start development server
npm run dev

# Build for production
npm run build

# Run linter
npm run lint
```

## Docker

When running via Docker Compose, the web panel is available at `http://localhost:3000`.

```bash
# Standalone mode
docker compose up -d

# Cluster mode
docker compose -f docker-compose.cluster.yml up -d
```

## Configuration

The web panel connects to the Oba REST API. In Docker, this is configured via nginx proxy.

For local development, update `vite.config.js` proxy settings if needed.

## Tech Stack

- React 18
- Vite
- Tailwind CSS
- Lucide React (icons)
