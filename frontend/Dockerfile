# Build stage
FROM node:20-alpine AS builder

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci

COPY . .

ARG NEXT_PUBLIC_API_URL=
ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL

RUN npm run build

# Production stage
FROM node:20-alpine AS runner

ARG VERSION=dev
ARG COMMIT=unknown

LABEL org.opencontainers.image.title="OmniCraft frontend" \
	org.opencontainers.image.description="OmniCraft Next.js web client" \
	org.opencontainers.image.source="https://github.com/omnicraft/omnicraft" \
	org.opencontainers.image.version="$VERSION" \
	org.opencontainers.image.revision="$COMMIT"

WORKDIR /app

ENV NODE_ENV=production

# Upgrade base packages (openssl security fixes) and drop the npm CLI whose
# bundled dependencies are not needed at runtime.
RUN apk upgrade --no-cache \
  && rm -rf /usr/local/lib/node_modules/npm /usr/local/bin/npm /usr/local/bin/npx

RUN addgroup --system --gid 1001 nodejs
RUN adduser --system --uid 1001 nextjs

COPY --from=builder /app/public ./public
COPY --from=builder --chown=nextjs:nodejs /app/.next/standalone ./
COPY --from=builder --chown=nextjs:nodejs /app/.next/static ./.next/static

USER nextjs

EXPOSE 3000

ENV PORT=3000
ENV HOSTNAME="0.0.0.0"

CMD ["node", "server.js"]
