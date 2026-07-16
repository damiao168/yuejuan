class AgentError(Exception):
    def __init__(self, code, message, status=500, retryable=False, request_id=""):
        super().__init__(message)
        self.code = code
        self.message = message
        self.status = status
        self.retryable = retryable
        self.request_id = request_id

    def payload(self):
        return {
            "schema_version": "grading-agent-v1",
            "request_id": self.request_id,
            "error": {
                "code": self.code,
                "message": self.message,
                "retryable": self.retryable,
            },
        }
