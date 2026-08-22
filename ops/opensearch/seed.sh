#!/bin/sh
set -eu

base_url="${OPENSEARCH_URL:-http://opensearch:9200}"
index="${OPENSEARCH_INDEX:-omnicraft-rag-v1}"
alias="${OPENSEARCH_ALIAS:-omnicraft-rag-read}"
mapping='{"mappings":{"dynamic":"strict","properties":{"chunk_key":{"type":"keyword"},"content_id":{"type":"long"},"content_version":{"type":"integer"},"chunk_index":{"type":"integer"},"chunking_version":{"type":"integer"},"index_version":{"type":"integer"},"embedding_model":{"type":"keyword"},"title":{"type":"text"},"heading":{"type":"text"},"text":{"type":"text"},"source_start":{"type":"integer"},"source_end":{"type":"integer"},"zone":{"type":"keyword"},"content_type":{"type":"keyword"},"category":{"type":"keyword"},"tags":{"type":"keyword"},"status":{"type":"keyword"},"ip":{"type":"long"}}}}'

status="$(curl -sS -o /dev/null -w '%{http_code}' "${base_url}/${index}")"
case "$status" in
  200)
    response="$(curl -fsS "${base_url}/${index}/_mapping")"
    printf '%s' "$response" | grep -Fq '"dynamic":"strict"'
    check_field() {
      printf '%s' "$response" | grep -Fq "\"$1\":{\"type\":\"$2\"}"
    }
    check_field chunk_key keyword
    check_field content_id long
    check_field content_version integer
    check_field chunk_index integer
    check_field chunking_version integer
    check_field index_version integer
    check_field embedding_model keyword
    check_field title text
    check_field heading text
    check_field text text
    check_field source_start integer
    check_field source_end integer
    check_field zone keyword
    check_field content_type keyword
    check_field category keyword
    check_field tags keyword
    check_field status keyword
    check_field ip long
    ;;
  404)
    curl -fsS -X PUT -H 'Content-Type: application/json' --data "$mapping" "${base_url}/${index}" >/dev/null
    ;;
  *)
    echo "unexpected OpenSearch index status: ${status}" >&2
    exit 1
    ;;
esac

curl -fsS -X POST -H 'Content-Type: application/json' \
  --data "{\"actions\":[{\"remove\":{\"index\":\"*\",\"alias\":\"${alias}\",\"must_exist\":false}},{\"add\":{\"index\":\"${index}\",\"alias\":\"${alias}\"}}]}" \
  "${base_url}/_aliases" >/dev/null
curl -fsS "${base_url}/_alias/${alias}" | grep -Fq "\"${index}\""

echo "OpenSearch seed ready: ${index} (alias=${alias})"
