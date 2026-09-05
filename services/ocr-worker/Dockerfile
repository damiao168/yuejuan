ARG OCR_PYTHON_IMAGE=python:3.11.15-slim-trixie
FROM ${OCR_PYTHON_IMAGE}

ARG INSTALL_PADDLEOCR=true
ARG INSTALL_PADDLEOCR_HPI=false
ARG PIP_INDEX_URL=https://pypi.org/simple
ARG DEBIAN_MIRROR=http://deb.debian.org/debian
ARG DEBIAN_SECURITY_MIRROR=http://deb.debian.org/debian-security

ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1

WORKDIR /app

COPY services/ocr-worker/pyproject.toml ./pyproject.toml

RUN python -m pip install --no-cache-dir --index-url "$PIP_INDEX_URL" \
      "pip==26.1.2" "setuptools==83.0.0" \
    && if [ "$INSTALL_PADDLEOCR" = "true" ]; then \
      python -m pip install --no-cache-dir --index-url "$PIP_INDEX_URL" \
        "paddleocr==3.7.0" "paddlepaddle==3.3.1" "tokenizers==0.22.1" "Pillow>=10,<12"; \
      if [ "$INSTALL_PADDLEOCR_HPI" = "true" ]; then \
        paddlex --install hpi-cpu; \
      fi; \
    fi

COPY services/ocr-worker/ocr_worker ./ocr_worker
RUN python -m pip install --no-cache-dir --no-deps .

RUN sed -i \
      -e "s|http://deb.debian.org/debian-security|$DEBIAN_SECURITY_MIRROR|g" \
      -e "s|http://deb.debian.org/debian|$DEBIAN_MIRROR|g" \
      /etc/apt/sources.list.d/debian.sources \
    && apt-get update \
    && apt-get install -y --no-install-recommends libgomp1 libgl1 libglib2.0-0 \
    && rm -rf /var/lib/apt/lists/*

RUN install -d -o nobody -g nogroup \
      /home/ocr-worker/.paddlex \
      /home/ocr-worker/.modelscope \
      /home/ocr-worker/.cache \
      /home/ocr-worker/.cache/huggingface

# PaddleX initializes its cache while importing PaddleOCR. The runtime user is
# intentionally non-root, so give it a stable writable cache location. Keep
# these runtime-only variables after dependency installation so changing them
# does not invalidate the large Paddle image layer.
ENV HOME=/home/ocr-worker
ENV PADDLE_PDX_CACHE_HOME=/home/ocr-worker/.paddlex

USER nobody

CMD ["python", "-m", "ocr_worker"]
