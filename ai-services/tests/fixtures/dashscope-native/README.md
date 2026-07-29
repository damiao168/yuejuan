# DashScope native protocol fixtures

These files are synthetic, offline protocol specimens. They are not captured student data, do not contain credentials, and are never sent to Alibaba Cloud.

The request shape and native text-generation path are based on:

- <https://help.aliyun.com/zh/model-studio/text-generation>
- <https://help.aliyun.com/zh/model-studio/qwen-api-via-dashscope>
- <https://help.aliyun.com/zh/model-studio/qwen-structured-output>
- <https://help.aliyun.com/en/model-studio/vision>

The error specimens use the status/code semantics documented at:

- <https://help.aliyun.com/zh/model-studio/error-code/>

`error-*.json` wraps the native response body with `http_status` as local fixture metadata. This wrapper is not sent to or returned by DashScope.

The text model uses a dated protocol specimen and the image fixture uses a
stable model alias. Neither is an approved production deployment. Final model
selection and version pinning, account authorization, endpoint region, pricing
and retention approval remain deferred to STORY-061B1.

`request-image.json` contains a tiny synthetic PNG as an inline Data URL. The
multimodal contract intentionally refuses public URLs, local paths and OSS URLs,
and accepts exactly one server-attested `answer_segment_crop`.
