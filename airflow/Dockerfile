# =====================================================================
# Airflow image bundling everything needed to run BOTH pipelines:
#   * the Go ELT binary (Intermediate)
#   * the Python EL script + deps (Beginner)
#   * dbt-core + dbt-clickhouse
# Build context = repository root (see docker-compose.yml).
# =====================================================================

# ---------- stage 1: build the Go ELT binary ----------
FROM golang:1.21-bookworm AS gobuild
WORKDIR /src
ENV GOFLAGS=-mod=mod
COPY pipeline/golang/go.mod ./
COPY pipeline/golang/ ./
RUN CGO_ENABLED=0 go build -o /out/elt . \
 && CGO_ENABLED=0 go build -o /out/elt-advanced ./cmd/advanced

# ---------- stage 2: airflow runtime ----------
FROM apache/airflow:2.7.3-python3.11

USER root
# Go binaries used by the intermediate + advanced DAGs
COPY --from=gobuild /out/elt /usr/local/bin/elt
COPY --from=gobuild /out/elt-advanced /usr/local/bin/elt-advanced
RUN chmod +x /usr/local/bin/elt /usr/local/bin/elt-advanced
USER airflow

# Python deps (dbt + connectors)
COPY airflow/requirements.txt /tmp/requirements.txt
RUN pip install --no-cache-dir -r /tmp/requirements.txt

# Project code baked into the image
COPY --chown=airflow:root dbt /opt/airflow/dbt
COPY --chown=airflow:root pipeline/python /opt/airflow/pipeline/python
COPY --chown=airflow:root pipeline/golang/config /opt/airflow/pipeline/golang/config

ENV DBT_PROFILES_DIR=/opt/airflow/dbt \
    DBT_PROJECT_DIR=/opt/airflow/dbt \
    PYTHONPATH=/opt/airflow/pipeline/python
