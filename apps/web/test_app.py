import unittest
from unittest.mock import patch

import app
from werkzeug.exceptions import HTTPException


class WebTest(unittest.TestCase):
    def setUp(self):
        self.client = app.app.test_client()

    def test_health(self):
        self.assertEqual(self.client.get("/healthz").status_code, 200)

    @patch("app.call_votes", return_value={"id": "test-id"})
    def test_create_redirects_to_shareable_poll(self, _):
        response = self.client.post(
            "/polls",
            data={"question": "Tea?", "choices": ["Yes", "No"]},
        )
        self.assertEqual(response.status_code, 302)
        self.assertEqual(response.headers["Location"], "/polls/test-id")

    @patch("app.requests.request")
    def test_votes_client_error_is_preserved(self, request):
        request.return_value.status_code = 400
        request.return_value.json.return_value = {"error": "invalid poll"}
        with app.app.test_request_context(), self.assertRaises(HTTPException) as error:
            app.call_votes("POST", "/polls")
        self.assertEqual(error.exception.code, 400)


if __name__ == "__main__":
    unittest.main()
