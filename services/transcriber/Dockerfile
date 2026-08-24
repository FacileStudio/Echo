FROM python:3.12-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ffmpeg ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY agent.py .

ENV VOSK_MODEL_PATH=/app/models/vosk-model-small-fr-0.22
# The model is downloaded at first boot (persist it in a volume to skip the wait).
CMD ["sh", "-c", "test -d $VOSK_MODEL_PATH || python agent.py download-files; python agent.py start"]
