FROM python:3.11-slim

WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY raw_schema.py extract_load.py ./

ENTRYPOINT ["python", "extract_load.py"]
