class NotificationService:
    """Secondary actor (mocked): records messages instead of sending them."""

    def __init__(self):
        self.sent = []

    def notify(self, user, message):
        self.sent.append((user, message))
