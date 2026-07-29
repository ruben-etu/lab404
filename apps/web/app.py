import logging
import os

import requests
from flask import Flask, abort, redirect, render_template, request, url_for

app = Flask(__name__)
logger = logging.getLogger("web")
votes_url = os.getenv("VOTES_URL", "http://localhost:8081")


def call_votes(method, path, **kwargs):
    try:
        response = requests.request(
            method, f"{votes_url}{path}", timeout=3, **kwargs
        )
    except requests.RequestException:
        logger.exception("votes service request failed", extra={"path": path})
        abort(502, "The voting service is unavailable.")
    if response.status_code >= 400:
        message = response.json().get("error", "Request failed.")
        abort(response.status_code if response.status_code < 500 else 502, message)
    return response.json() if response.content else None


@app.get("/")
def home():
    return render_template("home.html")


@app.post("/polls")
def create_poll():
    poll = call_votes(
        "POST",
        "/polls",
        json={
            "question": request.form.get("question", ""),
            "choices": request.form.getlist("choices"),
        },
    )
    logger.info("poll created", extra={"poll_id": poll["id"]})
    return redirect(url_for("show_poll", poll_id=poll["id"]))


@app.get("/polls")
def list_polls():
    return render_template("polls.html", polls=call_votes("GET", "/polls"))


@app.get("/polls/<poll_id>")
def show_poll(poll_id):
    return render_template(
        "poll.html",
        poll=call_votes("GET", f"/polls/{poll_id}"),
        share_url=request.url,
    )


@app.post("/polls/<poll_id>/votes")
def vote(poll_id):
    choice = request.form.get("choice", "")
    if not choice.isdigit():
        abort(400, "A valid choice is required.")
    call_votes(
        "POST",
        f"/polls/{poll_id}/votes",
        json={"choice": int(choice)},
    )
    logger.info("vote recorded", extra={"poll_id": poll_id})
    return redirect(url_for("show_poll", poll_id=poll_id))


@app.get("/healthz")
def health():
    return {"status": "ok"}
