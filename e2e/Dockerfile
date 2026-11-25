FROM mcr.microsoft.com/playwright:v1.64.1-noble
WORKDIR /suite
RUN npm install --no-audit --no-fund --save-exact @playwright/test@1.64.1
COPY *.mjs ./
CMD ["npx", "playwright", "test", "--workers=1"]
