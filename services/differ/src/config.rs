//! Service configuration loaded from the environment.

/// Differ runtime config. The only knob is the listen port; the service is
/// otherwise stateless and self-contained.
#[derive(Debug, Clone)]
pub struct Config {
    pub port: u16,
}

const DEFAULT_PORT: u16 = 9090;

impl Config {
    /// Read config from the environment. `DIFFER_PORT` overrides the default; a
    /// malformed value falls back to the default with a warning at the call site.
    pub fn from_env() -> Self {
        let port = std::env::var("DIFFER_PORT")
            .ok()
            .and_then(|v| v.parse::<u16>().ok())
            .unwrap_or(DEFAULT_PORT);
        Self { port }
    }
}

impl Default for Config {
    fn default() -> Self {
        Self { port: DEFAULT_PORT }
    }
}
