ARG AI_PYTHON_IMAGE=python:3.12-alpine
FROM ${AI_PYTHON_IMAGE}

WORKDIR /app
RUN addgroup -S edugrade && adduser -S -G edugrade -u 10001 edugrade

COPY ai-services/grading_agent /app/grading_agent
COPY ai-services/prompts /app/prompts
COPY contracts/grading-agent /app/contracts/grading-agent

RUN chown -R edugrade:edugrade /app
USER edugrade

ENV EDUGRADE_GRADING_AGENT_HOST=0.0.0.0 \
    EDUGRADE_GRADING_AGENT_PORT=8100 \
    EDUGRADE_GRADING_CONTRACT_ROOT=/app/contracts/grading-agent/v1 \
    EDUGRADE_GRADING_PROMPT_ROOT=/app/prompts \
    PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1
EXPOSE 8100
CMD ["python", "-m", "grading_agent"]
