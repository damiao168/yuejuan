ARG OCR_PYTHON_IMAGE=python:3.11-slim
FROM ${OCR_PYTHON_IMAGE}

ARG INSTALL_PADDLEOCR=true

ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1

WORKDIR /app

COPY services/ocr-worker/pyproject.toml ./pyproject.toml
COPY services/ocr-worker/ocr_worker ./ocr_worker

RUN if [ "$INSTALL_PADDLEOCR" = "true" ]; then \
      pip install --no-cache-dir ".[paddle]"; \
    else \
      pip install --no-cache-dir .; \
    fi

USER nobody

CMD ["python", "-m", "ocr_worker"]
