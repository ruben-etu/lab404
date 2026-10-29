FROM dhi.io/python:3.14-alpine3.24-dev@sha256:455286c3c9bea72bc093f9ba861c6ddbeac1923f12c5f5c71eafc4271819baca AS build

WORKDIR /app
RUN python -m venv /venv
ENV PATH="/venv/bin:$PATH"
COPY requirements.lock .
RUN pip install --no-cache-dir --require-hashes -r requirements.lock \
    && python -m pip uninstall --yes pip

FROM dhi.io/python:3.14-alpine3.24@sha256:00f9aeadee003dec0cafd5ebd3b36e668ac2a4e120e0c059967ad249e1130179

WORKDIR /app
COPY --from=build /venv /venv
ENV PATH="/venv/bin:$PATH"
COPY app.py .
COPY templates ./templates

USER 65532:65532
EXPOSE 8080
CMD ["opentelemetry-instrument", "gunicorn", "--bind", "0.0.0.0:8080", "--access-logfile", "-", "app:app"]
