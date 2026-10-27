FROM dhi.io/python:3.14-dev AS build

WORKDIR /app
RUN python -m venv /venv
ENV PATH="/venv/bin:$PATH"
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

FROM dhi.io/python:3.14

WORKDIR /app
COPY --from=build /venv /venv
ENV PATH="/venv/bin:$PATH"
COPY . .

EXPOSE 8080
CMD ["opentelemetry-instrument", "gunicorn", "--bind=0.0.0.0:8080", "--access-logfile=-", "app:app"]

