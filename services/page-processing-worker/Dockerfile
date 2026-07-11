ARG PAGE_PROCESSING_PYTHON_IMAGE=python:3.11-slim
FROM ${PAGE_PROCESSING_PYTHON_IMAGE}

ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1

WORKDIR /app

COPY services/page-processing-worker/pyproject.toml ./pyproject.toml
COPY services/page-processing-worker/page_processing ./page_processing
RUN pip install --no-cache-dir .

USER nobody
CMD ["python", "-m", "page_processing"]
