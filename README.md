# Prediplay

Prediplay is a full-stack football analytics application. It pulls player statistics from the Bzzoiro sports API for the top 5 major leagues, runs predictions and scoring, and serves the results through a REST API consumed by an Angular frontend.

Scoring highlights:

- Top players
- Benchwarmers
- Hidden gems
- Red flags

## Project Structure

```
prediplay/
├── backend/    # Go API server (chi router, SQLite via GORM)
└── frontend/   # Angular 17 web app
```

## Prerequisites

- [Go 1.22+](https://go.dev/dl/)
- [Node.js 18+ and npm](https://nodejs.org/)
- [Angular CLI](https://angular.io/cli): `npm install -g @angular/cli`

## Backend

### Configuration

Create a `.env` file in `backend/`:

```env
BZZOIRO_API_TOKEN=your_token_here
BZZOIRO_BASE_URL=https://sports.bzzoiro.com
DATABASE_PATH=./prediplay_fresh.db
PORT=8080
```

`BZZOIRO_API_TOKEN` is required. The other values have defaults and are optional.

### Run

```bash
cd backend
go run .
```

The server starts on `http://localhost:8080`. Player sync runs in the background on startup.

### API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/leagues` | All leagues |
| GET | `/api/teams` | All teams |
| GET | `/api/events` | Upcoming events |
| GET | `/api/live` | Live events |
| GET | `/api/players` | All players |
| GET | `/api/players/{id}/photo` | Player photo |
| GET | `/api/players/{id}/stats` | Player stats |
| GET | `/api/predictions` | All predictions |
| GET | `/api/predict/player/{id}` | Prediction for a player |
| GET | `/api/predict/top` | Top predicted players |
| GET | `/api/predict/redflags` | Red flag players |
| GET | `/api/predict/benchwarmers` | Benchwarmer players |
| GET | `/api/predict/synergy` | Team synergy analysis |
| GET | `/api/predict/momentum` | Momentum analysis |
| GET | `/api/dashboard` | Dashboard summary |

## Frontend

```bash
cd frontend
npm install
ng serve
```

The app runs on `http://localhost:4200` and proxies API calls to the backend at `http://localhost:8080`.

## Development

Run backend and frontend in separate terminals:

**Terminal 1 — backend:**
```bash
cd backend && go run .
```

**Terminal 2 — frontend:**
```bash
cd frontend && ng serve
```
