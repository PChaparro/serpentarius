# Serpentarius

<div align="center">
  <img src="docs/assets/logo.png" alt="Serpentarius Logo" width="200px" height="200px" />
</div>

[![Apache-2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Reference](https://pkg.go.dev/badge/github.com/PChaparro/serpentarius.svg)](https://pkg.go.dev/github.com/PChaparro/serpentarius)
[![Go Report Card](https://goreportcard.com/badge/github.com/PChaparro/serpentarius)](https://goreportcard.com/report/github.com/PChaparro/serpentarius)

Serpentarius (aka "Secretary Bird") is a REST microservice that generates PDF documents from HTML templates for your projects.

## What problems does Serpentarius solve? 🤔

Generating PDFs from HTML is a common practice due to its flexibility (you can create almost any design using HTML and CSS). However, integrating this functionality into each project presents several challenges:

- **Browser installation requirements**: You need Chromium to render HTML, which significantly increases Docker image sizes and resource consumption on your servers 💸.

- **High resource consumption**: PDF generation is computationally intensive. With a dedicated microservice, you can scale this functionality independently 🚀.

- **Cache optimization**: Serpentarius implements Redis caching to avoid repeatedly generating the same document, a feature you wouldn't want to implement in each of your projects.

Serpentarius solves these problems by exposing a REST API that you can query from any project or programming language.

## What Serpentarius does **NOT** solve ❌

- Serpentarius does **not** store HTML templates. Your project must handle this and send the HTML with each request. This makes Serpentarius agnostic regarding technologies and programming languages—**you only need to send valid HTML and Serpentarius will convert it to PDF**.
- Serpentarius does **not** optimize the HTML it receives. Your project should apply best practices such as using appropriately sized images, avoiding heavy fonts, and eliminating unnecessary styles. This helps reduce the size of the resulting PDF and ensures better performance. Serpentarius renders exactly what it receives and does **not** make modifications to avoid unexpected results.

## Installation ⬇️

This project is designed to function as a REST microservice, not as a library.

You can use Docker or compile the project to run it. In both cases, you'll need:

- Storage compatible with S3 API (to save generated documents) 📂
- Server compatible with Redis API (for caching) ⚡
- Chromium or similar browser (for rendering documents) 🖥️

### Build 🛠️

To compile the project, you need [Go](https://golang.org/dl/).

Once you've cloned the project, compile it with:

```bash
go build -o serpentarius.bin cmd/http/main.go
```

This will generate the `serpentarius.bin` binary in the project root, using the entry point `cmd/http/main.go`.

To run it:

```bash
./serpentarius.bin
```

### Docker 🐳

With the project cloned, build the Docker image:

```bash
docker build -f Containerfile -t serpentarius .
```

And run the container:

```bash
docker run -p 3000:3000 -e AWS_S3_ENDPOINT_URL=http://localhost:9000 \
  -e AWS_ACCESS_KEY_ID=value \
  -e AWS_SECRET_ACCESS_KEY=value \
  -e AWS_REGION=us-east-1 \
  -e REDIS_HOST=localhost \
  -e REDIS_PORT=6379 \
  -e REDIS_PASSWORD=dragonfly \
  -e REDIS_DB=0 \
  -e AUTH_SECRET=your_secret_key \
  -e CHROMIUM_BINARY_PATH=/usr/bin/chromium \
  -e MAX_CHROMIUM_BROWSERS=1 \
  -e MAX_CHROMIUM_TABS_PER_BROWSER=4 \
  -e MAX_IDLE_SECONDS=30 \
  -e ENVIRONMENT=production \
  serpentarius
```

### Environment Variables 🌍

For any installation method, configure these variables in a `.env` file or in the environment:

| Name                            | Description                                                | Development Value                                                                    |
| ------------------------------- | ---------------------------------------------------------- | ------------------------------------------------------------------------------------ |
| `AWS_S3_ENDPOINT_URL`           | S3 endpoint URL                                            | `http://localhost:9000`                                                              |
| `AWS_ACCESS_KEY_ID`             | AWS access key ID                                          | Create a Bucket and copy the `Access Key ID` of a user with access to the Bucket     |
| `AWS_SECRET_ACCESS_KEY`         | AWS secret access key                                      | Create a Bucket and copy the `Secret Access Key` of a user with access to the Bucket |
| `AWS_REGION`                    | AWS region where the Bucket is located                     | Default value is `us-east-1`                                                         |
| `REDIS_HOST`                    | Redis server hostname                                      | `localhost`                                                                          |
| `REDIS_PORT`                    | Redis server port                                          | `6379`                                                                               |
| `REDIS_PASSWORD`                | Redis server password                                      | `dragonfly`                                                                          |
| `REDIS_DB`                      | Redis database to use                                      | `0`                                                                                  |
| `AUTH_SECRET`                   | Secret key for user authentication                         | No default value                                                                     |
| `CHROMIUM_BINARY_PATH`          | Path to the Chromium binary                                | `/usr/bin/chromium`                                                                  |
| `MAX_CHROMIUM_BROWSERS`         | Maximum number of concurrent Chromium browsers             | `1`                                                                                  |
| `MAX_CHROMIUM_TABS_PER_BROWSER` | Maximum number of tabs per Chromium browser                | `4`                                                                                  |
| `MAX_IDLE_SECONDS`              | Maximum seconds a page can remain idle before being closed | `30`                                                                                 |
| `ENVIRONMENT`                   | Execution environment (development/production)             | `development`                                                                        |

The values shown in the `Development Value` column are compatible with the `container-compose.yml` file included in the project, which configures Dragonfly (Redis alternative) and MinIO (S3 alternative) for local development. If you use your own servers, adjust these variables accordingly.

To generate the authentication secret, you can use the following command:

```bash
openssl rand -base64 64
```
