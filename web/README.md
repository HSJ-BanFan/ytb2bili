# Bili-Up Web

Next.js frontend for the public YTB2BILI video-processing tool.

## Features

- Submit and monitor videos and task steps
- Manage Bilibili upload accounts through QR login
- Configure global AI, download, and upload settings
- Review subtitles, metadata, schedules, and generated files
- Responsive Tailwind CSS interface

## Development

```bash
npm install
npm run dev
```

Open <http://localhost:3000>. The development proxy forwards `/api` to the
backend at `http://localhost:8096`.

## Production build

```bash
npm run build
npm start
```

The backend serves the exported frontend from `static/` after the project's
build process copies the generated files.

## API boundary

Application routes are public. The `/api/v1/auth/*` routes are only for
Bilibili credential and account management; they are not application login.
