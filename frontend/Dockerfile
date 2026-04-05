FROM node:20-alpine AS builder

WORKDIR /app

COPY frontend/package.json ./package.json
RUN npm install

COPY frontend/ ./
RUN npm run build

FROM alpine:3.20

WORKDIR /app

COPY --from=builder /app/build ./build

CMD ["sh", "-c", "printf 'frontend build available at /app/build\n'"]
