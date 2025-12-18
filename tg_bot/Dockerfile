FROM node:20-bookworm-slim

WORKDIR /app

# + fonts for Cyrillic rendering in SVG->PNG (sharp/libvips)
RUN apt-get update \
  && apt-get install -y --no-install-recommends \
     ca-certificates tzdata fontconfig fonts-dejavu-core fonts-noto-core \
  && rm -rf /var/lib/apt/lists/*

COPY tg_bot/package.json tg_bot/package-lock.json* ./

RUN if [ -f package-lock.json ]; then \
      npm ci; \
    else \
      npm install; \
    fi \
 && npm prune --omit=dev

COPY tg_bot/src ./src

ENV NODE_ENV=production
CMD ["node", "src/index.js"]