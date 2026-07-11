from page_processing.api import Client
from page_processing.config import load_config
from page_processing.runner import Runner


def main() -> None:
    config = load_config()
    if not config.username or not config.password:
        raise SystemExit("page-processing worker credentials are required")
    client = Client(config.base_url, config.tenant_code, config.username, config.password)
    client.login()
    Runner(client, config).run_forever()


if __name__ == "__main__":
    main()
