ARG PAGE_PROCESSING_PYTHON_IMAGE=python:3.11.15-slim-trixie
FROM ${PAGE_PROCESSING_PYTHON_IMAGE}

# Custom runtime bases may default to an unprivileged user. Package
# installation must run as root; the final process is dropped back to nobody.
USER root

# Wheel downloads for the image-processing stack are sizeable. Keep failures
# visible, but tolerate transient slow reads from the package index in CI and
# developer Docker environments.
ARG PIP_RETRIES=5
ARG PIP_TIMEOUT=120

ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1

WORKDIR /app

COPY services/page-processing-worker/pyproject.toml ./pyproject.toml
COPY services/page-processing-worker/page_processing ./page_processing
RUN python -m pip install --no-cache-dir --retries "${PIP_RETRIES}" --timeout "${PIP_TIMEOUT}" \
      "pip==26.1.2" "setuptools==83.0.0" \
    && python -m pip install --no-cache-dir --retries "${PIP_RETRIES}" --timeout "${PIP_TIMEOUT}" .

USER nobody
CMD ["python", "-m", "page_processing"]
