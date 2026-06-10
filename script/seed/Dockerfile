FROM python:3.11-slim
WORKDIR /seed
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY generate_seed.py .
ENTRYPOINT ["python", "generate_seed.py"]
