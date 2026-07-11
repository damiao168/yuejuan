ARG QUALITY_PYTHON_IMAGE=python:3.11-slim
FROM ${QUALITY_PYTHON_IMAGE}

ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1

WORKDIR /app

COPY services/image-quality-worker/pyproject.toml ./pyproject.toml
RUN pip install --no-cache-dir --timeout 180 --retries 10 \
    "numpy>=1.26" "opencv-python-headless>=4.9" "pillow>=10.0"

COPY services/image-quality-worker/image_quality ./image_quality
RUN pip install --no-cache-dir --no-deps .

USER nobody

CMD python -m image_quality
