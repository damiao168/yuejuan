ARG AI_PYTHON_IMAGE=python:3.12-alpine
FROM ${AI_PYTHON_IMAGE}

WORKDIR /app
COPY ai-services/placeholder_server.py /app/placeholder_server.py

ENV EDUGRADE_AI_PLACEHOLDER_PORT=8100
EXPOSE 8100
CMD ["python", "/app/placeholder_server.py"]
