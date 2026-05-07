use serde::{Deserialize, Serialize};
use url::Url;

/// Parameters extracted from omnicraft://deploy?content_id=X&token=Y
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct UrlSchemeParams {
    pub content_id: String,
    pub token: Option<String>,
}

/// Parse omnicraft:// protocol URLs from command-line arguments.
///
/// On Windows, when a URL scheme is triggered, the full URL is passed as the
/// first argument to the executable. We extract the query parameters from it.
///
/// Expected format: `omnicraft://deploy?content_id=123&token=abc`
pub fn parse_deploy_args(args: &[String]) -> Option<UrlSchemeParams> {
    for arg in args.iter().skip(1) {
        // Skip Tauri/Electron internal flags
        if arg.starts_with("--") || arg.starts_with('-') {
            continue;
        }

        if let Some(params) = parse_omnicraft_url(arg) {
            return Some(params);
        }
    }
    None
}

fn parse_omnicraft_url(raw: &str) -> Option<UrlSchemeParams> {
    // Normalize: Windows may pass the URL without protocol separator
    let url_str = if raw.starts_with("omnicraft:") {
        raw.to_string()
    } else if raw.starts_with("omnicraft://") {
        raw.to_string()
    } else {
        return None;
    };

    let url = Url::parse(&url_str).ok()?;

    if url.host_str()? != "deploy" {
        return None;
    }

    let mut params = url.query_pairs();

    let content_id = params
        .find(|(k, _)| k == "content_id")
        .map(|(_, v)| v.to_string())?;

    let token = params
        .find(|(k, _)| k == "token")
        .map(|(_, v)| v.to_string());

    Some(UrlSchemeParams { content_id, token })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_parse_valid_deploy_url() {
        let args = vec![
            "omnicraft.exe".to_string(),
            "omnicraft://deploy?content_id=42&token=abc123".to_string(),
        ];
        let params = parse_deploy_args(&args).unwrap();
        assert_eq!(params.content_id, "42");
        assert_eq!(params.token.unwrap(), "abc123");
    }

    #[test]
    fn test_parse_url_without_token() {
        let args = vec![
            "omnicraft.exe".to_string(),
            "omnicraft://deploy?content_id=100".to_string(),
        ];
        let params = parse_deploy_args(&args).unwrap();
        assert_eq!(params.content_id, "100");
        assert!(params.token.is_none());
    }

    #[test]
    fn test_parse_ignores_tauri_flags() {
        let args = vec![
            "omnicraft.exe".to_string(),
            "--some-tauri-flag".to_string(),
            "omnicraft://deploy?content_id=7".to_string(),
        ];
        let params = parse_deploy_args(&args).unwrap();
        assert_eq!(params.content_id, "7");
    }

    #[test]
    fn test_parse_non_omnicraft_url_returns_none() {
        let args = vec![
            "omnicraft.exe".to_string(),
            "https://example.com".to_string(),
        ];
        assert!(parse_deploy_args(&args).is_none());
    }

    #[test]
    fn test_parse_empty_args() {
        let args = vec!["omnicraft.exe".to_string()];
        assert!(parse_deploy_args(&args).is_none());
    }
}
