#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${NAMESPACE:-default}"
TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

# Discover CAPI workload clusters
clusters=$(kubectl get clusters -n "$NAMESPACE" -o jsonpath='{.items[*].metadata.name}')
if [[ -z "$clusters" ]]; then
  echo "No CAPI clusters found in namespace '$NAMESPACE'"
  exit 1
fi

echo "Found clusters: $clusters"
echo

for cluster in $clusters; do
  echo "--- $cluster ---"

  # Talosconfig
  if kubectl -n "$NAMESPACE" get secret "${cluster}-talosconfig" &>/dev/null; then
    kubectl -n "$NAMESPACE" get secret "${cluster}-talosconfig" \
      -o jsonpath='{.data.talosconfig}' | base64 -d > "$TMPDIR/${cluster}-talosconfig"
    talosctl config merge "$TMPDIR/${cluster}-talosconfig"
    echo "  talosconfig merged"
  else
    echo "  talosconfig secret not found, skipping"
  fi

  # Kubeconfig
  if kubectl -n "$NAMESPACE" get secret "${cluster}-kubeconfig" &>/dev/null; then
    kubectl -n "$NAMESPACE" get secret "${cluster}-kubeconfig" \
      -o jsonpath='{.data.value}' | base64 -d > "$TMPDIR/${cluster}-kubeconfig"
    kubectl konfig import -s "$TMPDIR/${cluster}-kubeconfig"
    echo "  kubeconfig imported"
  else
    echo "  kubeconfig secret not found, skipping"
  fi

  echo
done

echo "Done. Available talos contexts:"
talosctl config contexts
