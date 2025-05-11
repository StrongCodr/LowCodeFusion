# LowCodeFusion

LowCodeFusion is a Pulumi‑style SDK generator that transforms Pliant's automation‑library definitions into native modules for Python, Node.js, and beyond—providing a code‑first, imperative interface to automate infrastructure with full IDE autocomplete support. Requires a fully licensed Pliant instance to serve as the backend.

`lcf` is a Go-based CLI that downloads Pliant integration definitions and scaffolds language-specific SDKs. First target: Python.

## Usage

```bash
lcf download --integration AWS --lang python --out ./sdk
```

## Type Organization

The generated SDK follows a two-level type hierarchy:

1. **Common Types** (`_types/common_types.py`): Types shared across multiple services
2. **Service-Specific Types** (`_types/ec2/types.py`): Types specific to a single service

This approach reduces duplication while maintaining a clean, organized structure that's easy to navigate.

## Installation (dev)

```bash
go install github.com/strongcodr/lowcodefusion@latest
```
