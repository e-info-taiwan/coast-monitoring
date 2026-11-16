import contextlib
import importlib.util
import io
import json
from pathlib import Path
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location(
    "replace_reef", Path(__file__).resolve().parents[1] / "replace-reef-observations.py")
runner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runner)


class ConnectionTests(unittest.TestCase):
    def test_project_documented_keyword_format(self):
        result = runner.parse_database_connection(
            "user=coast password=fake dbname=coast_monitoring "
            "host=/cloudsql/tnf-rc:asia-east1:coast-monitoring-db sslmode=disable")
        self.assertEqual(result["user"], "coast")
        self.assertEqual(result["dbname"], "coast_monitoring")
        self.assertEqual(result["host"], "/cloudsql/tnf-rc:asia-east1:coast-monitoring-db")

    def test_keyword_quotes_escapes_and_equals(self):
        result = runner.parse_database_connection(
            r" user = coast password = 'a b\'c\\d=x' dbname=coast\ db sslmode='' ")
        self.assertEqual(result["password"], "a b'c\\d=x")
        self.assertEqual(result["dbname"], "coast db")
        self.assertEqual(result["sslmode"], "")
        self.assertEqual(runner.parse_database_connection("password=a=b=c")["password"], "a=b=c")

    def test_uri_and_query_parameters(self):
        result = runner.parse_database_connection(
            "postgresql://old:p%40ss%3Dword@/coast%20db?"
            "host=%2Fcloudsql%2Fproject%3Aregion%3Ainstance&user=coast")
        self.assertEqual(result["password"], "p@ss=word")
        self.assertEqual(result["host"], "/cloudsql/project:region:instance")
        self.assertEqual(result["user"], "coast")
        self.assertEqual(result["dbname"], "coast db")

    def test_duplicate_keys(self):
        self.assertEqual(runner.parse_database_connection("user=old user=new")["user"], "new")

    def test_invalid_strings_do_not_leak_input(self):
        for value in ["private-secret", "password='private-secret", "password=private-secret\\",
                      "password='private-secret'junk", "postgresql://[private-secret"]:
            with self.subTest(value=value), self.assertRaises(RuntimeError) as error:
                runner.parse_database_connection(value)
            self.assertNotIn("private-secret", str(error.exception))

    def test_cloud_target_keyword_secret_and_host_verification(self):
        instance = "tnf-rc:asia-east1:test-instance"
        service = {"spec": {"template": {
            "metadata": {"annotations": {"run.googleapis.com/cloudsql-instances": instance}},
            "spec": {"containers": [{"env": [{"name": "DATABASE_URL", "valueFrom": {
                "secretKeyRef": {"name": "test-secret", "key": "latest"}}}]}]}}}}
        secret = f"host=/cloudsql/{instance} user=coast password='fake secret' dbname=reef"
        with patch.object(runner, "run", side_effect=[json.dumps(service), secret, '{"ipAddresses":[]}']), \
                contextlib.redirect_stdout(io.StringIO()) as output:
            self.assertEqual(runner.cloud_target(), (instance, "coast", "fake secret", "reef"))
        self.assertNotIn("fake secret", output.getvalue())
        with patch.object(runner, "run", side_effect=[json.dumps(service),
                "host=wrong.example user=coast password=fake dbname=reef", '{"ipAddresses":[]}']):
            with self.assertRaisesRegex(RuntimeError, "does not match"):
                runner.cloud_target()


if __name__ == "__main__":
    unittest.main()
