use hmac::{Hmac, Mac};
use serde::{Deserialize, Serialize};
use sha2::Sha256;

type HmacSha256 = Hmac<Sha256>;

const HMAC_SECRET: &str = env!("AGENT_HMAC_SECRET");

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ActionScript {
    pub content_id: String,
    pub actions: Vec<Action>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Action {
    pub action: String,
    pub payload: serde_json::Value,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[allow(dead_code)]
pub struct SignedScript {
    pub script: ActionScript,
    pub signature: String,
}

/// Verify the HMAC-SHA256 signature of a Go-issued action script.
/// Returns the deserialized script if valid.
#[tauri::command]
pub fn verify_script_signature(script_json: String, signature: String) -> Result<ActionScript, String> {
    let payload = script_json.as_bytes();

    let mut mac = HmacSha256::new_from_slice(HMAC_SECRET.as_bytes())
        .map_err(|e| format!("HMAC init error: {}", e))?;

    mac.update(payload);

    let expected = hex::encode(mac.finalize().into_bytes());

    if signature.to_lowercase() != expected.to_lowercase() {
        return Err("signature verification failed".into());
    }

    serde_json::from_str::<ActionScript>(&script_json).map_err(|e| format!("invalid script JSON: {}", e))
}

#[cfg(test)]
mod tests {
    use super::*;
    use hmac::Mac;

    fn sign(payload: &str) -> String {
        let mut mac = HmacSha256::new_from_slice(HMAC_SECRET.as_bytes()).unwrap();
        mac.update(payload.as_bytes());
        hex::encode(mac.finalize().into_bytes())
    }

    #[test]
    fn test_valid_signature() {
        let script = ActionScript {
            content_id: "42".into(),
            actions: vec![],
        };
        let json = serde_json::to_string(&script).unwrap();
        let sig = sign(&json);
        let result = verify_script_signature(json, sig);
        assert!(result.is_ok());
        assert_eq!(result.unwrap().content_id, "42");
    }

    #[test]
    fn test_invalid_signature() {
        let script = ActionScript {
            content_id: "42".into(),
            actions: vec![],
        };
        let json = serde_json::to_string(&script).unwrap();
        let result = verify_script_signature(json, "badsig".into());
        assert!(result.is_err());
    }

    #[test]
    fn test_tampered_payload() {
        let script = ActionScript {
            content_id: "42".into(),
            actions: vec![],
        };
        let json = serde_json::to_string(&script).unwrap();
        let sig = sign(&json);
        let tampered = json.replace("42", "43");
        let result = verify_script_signature(tampered, sig);
        assert!(result.is_err());
    }
}
