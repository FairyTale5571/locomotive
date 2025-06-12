# locomotive

A Railway sidecar service for sending webhook events when new logs are received. Supports Discord, Datadog, Axiom, BetterStack and more!

## Configuration

Configuration is done through environment variables. See explanation and examples below.

## Generic Webhook Log Formats

**These are examples of what the body would look like in the POST request done by locomotive**

For Plaintext logs

```json
[
    {
        "_metadata": {
            "deploymentId": "577e2cf2-a1fc-4e0f-b352-2780bca73a94",
            "deploymentInstanceId": "bbdc4e76-7600-415f-9f6b-e425311cec51",
            "environmentId": "b5ce7ab5-96f1-4fa3-929b-fc883f89cbd1",
            "environmentName": "production",
            "projectId": "8a6502bf-6479-440c-a14f-78ecd52abf09",
            "projectName": "Railyard",
            "serviceId": "24335e07-e68b-498f-bc9e-1b9146436867",
            "serviceName": "Autorack"
        },
        "message": "Hello, World!",
        "level": "info",
        "severity": "info",
        "time": "2020-05-22T21:27:33Z",
        "_time": "2020-05-22T21:27:33Z",
        "dt": "2020-05-22T21:27:33Z",
        "datetime": "2020-05-22T21:27:33Z",
        "ts": "2020-05-22T21:27:33Z",
        "timestamp": "2020-05-22T21:27:33Z"
    }
]
```

For Structured JSON logs

```json
[
    {
        "_metadata": {
            "deploymentId": "5b7c81b35-1578-4eb8-8498-44f4f517b263",
            "deploymentInstanceId": "46cda6d4-f76c-45cb-8642-c7265949e497",
            "environmentId": "b5ce7ab5-96f1-4fa3-929b-fc883f89cbd1",
            "environmentName": "production",
            "projectId": "8a6502bf-6479-440c-a14f-78ecd52abf09",
            "projectName": "Railyard",
            "serviceId": "55b1755f-f2c6-4f24-8d51-0ed3754b253e",
            "serviceName": "Superliner"
        },
        "level": "info",
        "severity": "info",
        "message": "Hello, World!",
        "example_string": "foo bar",
        "example_int": 12345678,
        "example_float": 1.2345678,
        "example_int_slice": [123, 456, 789],
        "example_string_slice": ["hello", "world"],
        "example_group": {
            "example_grouped_int": 12345678,
            "example_grouped_string": "Hello, World!"
        },
        "time": "2020-05-22T21:27:33Z",
        "_time": "2020-05-22T21:27:33Z",
        "dt": "2020-05-22T21:27:33Z",
        "datetime": "2020-05-22T21:27:33Z",
        "ts": "2020-05-22T21:27:33Z",
        "timestamp": "2020-05-22T21:27:33Z"
    }
]
```

Grafana Loki Plaintext Log Example

```json
{
    "streams": [
        {
            "stream": {
                "deployment_id": "fb8172c8-a65d-48a4-9d1e-9d5ef986c9c3",
                "deployment_instance_id": "25dfeb9b-0097-4f91-820b-5dccc5009b1d",
                "project_id": "dce92382-c4e4-4923-bacd-3a5f7bcab337",
                "project_name": "Union Pacific Freight",
                "environment_id": "57d88ccb-8db9-4aef-957e-ecd94c41fdf8",
                "environment_name": "production",
                "service_id": "aa8ce660-dad0-4f7d-8921-46295d180c09",
                "service_name": "Dash 8",
                "severity": "error",
                "level": "error"
            },
            "values": [["1590182853000000000", "a plaintext message", {}]]
        }
    ]
}
```

Grafana Loki Structured Log Example

```json
{
    "streams": [
        {
            "stream": {
                "deployment_id": "fb8172c8-a65d-48a4-9d1e-9d5ef986c9c3",
                "deployment_instance_id": "25dfeb9b-0097-4f91-820b-5dccc5009b1d",
                "project_id": "dce92382-c4e4-4923-bacd-3a5f7bcab337",
                "project_name": "Union Pacific Freight",
                "environment_id": "57d88ccb-8db9-4aef-957e-ecd94c41fdf8",
                "environment_name": "production",
                "service_id": "aa8ce660-dad0-4f7d-8921-46295d180c09",
                "service_name": "Dash 8",
                "severity": "error",
                "level": "error"
            },
            "values": [
                [
                    "1590182853000000000",
                    "hello, world",
                    {
                        "float": "10.51",
                        "number": "10",
                        "string_value": "hello world",
                        "user": "null"
                    }
                ]
            ]
        }
    ]
}
```

**Notes:**

-   Metadata is gathered once when the locomotive starts, If a project/service/environment name has changed, the name in the metadata will not be correct until the locomotive is restarted.

-   The body will always be a JSON array containing one or more log objects.

-   Various common timestamp attributes are included in every log object to increase compatibility with external logging services. [ref 1](https://axiom.co/docs/send-data/ingest#timestamp-field), [ref 2](https://betterstack.com/docs/logs/http-rest-api/#sending-timestamps)

-   The default `Content-Type` for these POST requests is set to `application/json`

-   Structured log attributes sent to Grafana Loki must always be a string

## Usage Examples

### Auto-Discovery Mode (Recommended)

Monitor all services in a project/environment automatically:

```bash
RAILWAY_API_KEY=your_api_key
RAILWAY_PROJECT_ID=your_project_id
RAILWAY_ENVIRONMENT_ID=your_environment_id
DISCORD_WEBHOOK_URL=https://discord.com/api/webhooks/...
```

### Legacy Mode

Manually specify services to monitor:

```bash
RAILWAY_API_KEY=your_api_key
ENVIRONMENT_ID=your_environment_id
TRAIN=service_id_1,service_id_2,service_id_3
DISCORD_WEBHOOK_URL=https://discord.com/api/webhooks/...
```

All variables:

-   `RAILWAY_API_KEY` - Your Railway API key.

    -   Project level keys do not work.

-   `RAILWAY_PROJECT_ID` - The Railway project ID to monitor.

    -   Optional. When specified together with `RAILWAY_ENVIRONMENT_ID`, the locomotive will automatically discover all services in the project/environment instead of requiring manual specification via `TRAIN`.

-   `RAILWAY_ENVIRONMENT_ID` - The Railway environment ID to monitor.

    -   Optional when used with `RAILWAY_PROJECT_ID` for auto-discovery mode.
    
-   `ENVIRONMENT_ID` - The environment ID your service is in (legacy mode).

    -   Auto-filled to the current environment ID.
    -   **Legacy**: Use `RAILWAY_ENVIRONMENT_ID` instead for new deployments.

-   `TRAIN` - The ID of the service you want to monitor.

    -   Supports multiple service IDs, separated with a comma.
    -   **Optional when using auto-discovery mode**: If `RAILWAY_PROJECT_ID` and `RAILWAY_ENVIRONMENT_ID` are specified, all services in the project/environment will be automatically monitored.
    -   **Required in legacy mode**: When using `ENVIRONMENT_ID`, you must specify the services to monitor.

-   `DISCORD_WEBHOOK_URL` - The Discord webhook URL to send logs to.

    -   Optional.

-   `DISCORD_PRETTY_JSON` - Pretty print the RAW JSON object in Discord embeds.

-   `SLACK_WEBHOOK_URL` - The Slack webhook URL to send logs to.

    -   Optional.

-   `SLACK_PRETTY_JSON` - Pretty print the RAW JSON object in Slack embeds.

-   `SLACK_TAGS` - Tags to add to the Slack message.

    -   Supports multiple tags, separated with a comma.
    -   Optional.

-   `LOKI_INGEST_URL` - The Loki ingest URL to send logs to.

    -   Example with no authentication: `https://loki-instance.up.railway.app/loki/api/v1/push`
    -   Example with username/password authentication: `https://user:pass@loki-instance.up.railway.app/loki/api/v1/push`
    -   Optional.

-   `INGEST_URL` - The URL to send a generic request to.

    -   Example for Datadog: `INGEST_URL=https://http-intake.logs.datadoghq.com/api/v2/logs`
    -   Example for Axiom: `INGEST_URL=https://api.axiom.co/v1/datasets/DATASET_NAME/ingest`
    -   Example for BetterStack: `INGEST_URL=https://in.logs.betterstack.com`
    -   Optional.

-   `ADDITIONAL_HEADERS` - Any additional headers to be sent with the generic request.

    -   Useful for auth. In the format of a cookie. meaning each key value pair is split by a semi-colon and each key value is split by an equals sign.
    -   Example for Datadog: `ADDITIONAL_HEADERS=DD-API-KEY=<DD_API_KEY>;DD-APPLICATION-KEY=<DD_APP_KEY>`
    -   Example for Axiom/BetterStack: `ADDITIONAL_HEADERS=Authorization=Bearer API_TOKEN`

-   `REPORT_STATUS_EVERY` - Reports the status of the locomotive every 5 seconds.

    -   Default: 5s.
    -   Format must be in the Golang time.DurationParse format
        -   E.g. 10h, 5h, 10m, 5m 5s

-   `LOGS_FILTER` - Global log filter.

    -   Either ALL, INFO, ERROR, WARN or any custom combination of severity / level.
    -   Accepts multiple values, separated with a comma.
    -   Defaults to allowing all log levels.
    -   Optional.

-   `LOGS_FILTER_DISCORD`