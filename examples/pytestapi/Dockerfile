FROM python:3.12.2-alpine3.19

ENV \
  PYTHONUNBUFFERED=1 \
  PIP_DISABLE_PIP_VERSION_CHECK=on \
  PIP_DEFAULT_TIMEOUT=100 \
  POETRY_VIRTUALENVS_CREATE=false
WORKDIR /pytestapi

RUN \
  --mount=type=cache,target=/root/.cache/pip \
  pip install -U pip setuptools && \
  pip install poetry==1.8.2

COPY pyproject.toml poetry.lock .
COPY pytestapi ./pytestapi
COPY scripts ./scripts
RUN \
  --mount=type=cache,target=/root/.cache/pip \
  --mount=type=cache,target=/root/.cache/pypoetry \
  poetry install --no-interaction --no-ansi

CMD ["poetry", "run", "--quiet", "pytestapi-server"]
