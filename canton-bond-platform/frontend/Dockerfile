FROM node:22-alpine

WORKDIR /app
COPY package.json package-lock.json ./
RUN npm install

COPY . .

ENV BACKEND_HTTP=http://backend:8080

EXPOSE 3000

CMD ["npx", "vite", "--host", "0.0.0.0"]
