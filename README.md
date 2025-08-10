# Excalidraw Complete: A Self-Hosted Solution

Excalidraw Complete simplifies the deployment of Excalidraw, bringing an
all-in-one solution to self-hosting this versatile virtual whiteboard. Designed
for ease of setup and use, Excalidraw Complete integrates essential features
into a single Go binary. This solution encompasses:

- The intuitive Excalidraw frontend UI for seamless user experience.
- An integrated data layer ensuring fast and efficient data handling based on different data providers.
- A socket.io implementation to enable real-time collaboration among users.

The project goal is to alleviate the setup complexities traditionally associated with self-hosting Excalidraw, especially in scenarios requiring data persistence and collaborative functionalities.

## Excalidraw Complete helm

- Updated Excalidraw Complete to use .env, with support for changing configurations without the need for rebuilding.
- Added PostgreSQL support.
- 
**(to do)**
- Add support for ARM architecture.
- Create a helm chart, to make Excalidraw Complete deployable on Kubernetes
- Update dependencies and change excalidraw module dependencies, to latets. 

## QuickStart

To add Excalidraw Complete to your environment, follow these steps:

1. **Download the latest release binary:**
   Visit [the releases page](https://github.com/PatWie/excalidraw-complete/releases/) to find the download URL for the latest binary. Use `wget` to download it:

   ```bash
   wget <binary-download-url>
   chmod +x excalidraw-complete
   ```

2. **Run the binary:**
   Start the Excalidraw Complete server:

   ```bash
   ./excalidraw-complete
   ```

3. **Access the application:**
   Once launched, Excalidraw Complete is accessible at `localhost:3002`, ready for drawing and collaboration.

### Docker

1. **Clone the repository or download the docker compose file:**
    ```bash
    git clone https://github.com/jmsaRuc/excalidraw-complete-helm.git --recursive
    cd excalidraw-complete-helm
    ```
2. **Run the Docker container:**
   Start the Excalidraw Complete server using Docker:

    ```bash
    docker compose up
    ```

3. **Access the application:**
   Once launched, Excalidraw Complete is accessible at `localhost:3002`, ready for drawing and collaboration.

### Configuration

Excalidraw Complete adapts to your preferences with customizable storage solutions, adjustable via the `STORAGE_TYPE` environment variable:

Add `.env` file in the root directory and customize it according to your needs.

- **Filesystem:** Opt for `STORAGE_TYPE=filesystem` and define `LOCAL_STORAGE_PATH` to use a local directory.
- **SQLite:** Select `STORAGE_TYPE=sqlite` with `DATA_SOURCE_NAME` for local SQLite storage, including the option for `:memory:` for ephemeral data.
- **AWS S3:** Choose `STORAGE_TYPE=s3` and specify `S3_BUCKET_NAME` to leverage S3 bucket storage, ideal for cloud-based solutions.

#### Only With Docker:
- **PostgreSQL:** Opt for `STORAGE_TYPE=postgres` and provide the necessary PostgreSQL connection details via `POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, and `POSTGRES_DB`.

    ```bash
    docker compose --env-file .env --profile postgres up
    ```

These flexible configurations ensure Excalidraw Complete fits seamlessly into your existing setup, whether on-premise or in the cloud.

## Building from Source

### Docker

To build Excalidraw Complete with Docker, follow these steps:

1. **Clone the repository:**
   ```bash
   git clone https://github.com/PatWie/excalidraw-complete.git --recursive
   cd ./excalidraw-complete
   ```
2. **Change imagename**
   In the `docker-compose.yaml` file, change the image name to your desired name (e.g., `my-excalidraw-complete`).

3. **Build the Docker image:**
   ```bash
   docker compose build
   ```
4. **Configure environment variables**
   Create a `.env` file in the root directory and customize it according to your needs.

5. **Run the Docker container:**
   ```bash
    docker compose --env-file .env up
   ```
6. **For postgres**
   If you're using PostgreSQL, make sure to start the PostgreSQL service with the following command:

    *.env:*
    ```shell
    STORAGE_TYPE=postgres
     # Adjust this URL if you're using a reverse proxy or different domain, if https:// aplication will use ssl
     #(e.g., `https://my-excalidraw-complete.com`)
    VITE_FRONTEND_URL=http://localhost:3002
    HOST=0.0.0.0
    PORT=3002
    LOG_LEVEL=info
     # default values
    POSTGRES_HOST=excalidraw-postgres
    POSTGRES_PORT=5432
    POSTGRES_USER=excalidraw
    POSTGRES_PASSWORD=excalidraw
    POSTGRES_DB=excalidraw
    ```

    ```bash
    docker compose --env-file .env --profile postgres up
    ```

### Go

Interested in contributing or customizing? Build Excalidraw Complete from source with these steps:

```bash
# Clone and prepare the Excalidraw frontend
git clone https://github.com/jmsaRuc/excalidraw-complete-helm --recursive
cd ./excalidraw-complete-helm

docker build -t exalidraw-ui-build excalidraw -f ui-build.Dockerfile
docker run -v ${PWD}/:/pwd/ -it exalidraw-ui-build cp -r /frontend /pwd
```

Compile the Go application:

```bash
go build -o excalidraw-complete main.go
```

Declare environment variables
Example: `STORAGE_TYPE=sqlite DATA_SOURCE_NAME=/tmp/excalidb.sqlite PORT=3002 HOST=0.0.0.0 LOG_LEVEL=info VITE_FRONTEND_URL=http://localhost:3002`

Start the server:

```bash
go run main.go

STORAGE_TYPE=sqlite DATA_SOURCE_NAME=test.db go run main.go

# or with filesystem storage
STORAGE_TYPE=filesystem LOCAL_STORAGE_PATH=/tmp/excalidraw/  go run main.go
```

Excalidraw Complete is now running on your machine, ready to bring your collaborative whiteboard ideas to life.

---

Excalidraw is a fantastic tool, but self-hosting it can be tricky. I welcome
your contributions to improve Excalidraw Complete — be it through adding new
features, improving existing ones, or bug reports.
