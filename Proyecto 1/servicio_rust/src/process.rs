use serde::{Deserialize, Serialize};

#[derive(Debug, Serialize, Deserialize)]
pub struct SystemInfo {
    #[serde(rename = "SystemMetrics")]
    pub metrics: SystemMetrics,
    #[serde(rename = "ProcessDetails")]
    pub processes: Vec<Process>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct SystemMetrics {
    #[serde(rename = "TotalMemory")]
    pub total_memory: i32,
    #[serde(rename = "FreeMemory")]
    pub free_memory: i32,
    #[serde(rename = "UsedMemory")]
    pub used_memory: i32,
    #[serde(rename = "CpuUsage")]
    pub cpu_usage: f32,
}

#[derive(Debug, Serialize, Deserialize, PartialEq)]
pub struct Process {
    #[serde(rename = "pid")]
    pub pid: i32,
    #[serde(rename = "name")]
    pub name: String,
    #[serde(rename = "ContainerId")]
    pub container_id: String,
    #[serde(rename = "MemoryUsage")]
    pub memory_usage: f32,
    #[serde(rename = "CpuUsage")]
    pub cpu_usage: f32,
    #[serde(rename = "DiskUsage")]
    pub disk_usage: i32,
    #[serde(rename = "IoActivity")]
    pub io_activity: i32,
    #[serde(skip_serializing_if = "Option::is_none", default)]
    pub creation_date: Option<String>,  // Nuevo campo, opcional
}

#[derive(Debug, Serialize, Clone)]
pub struct LogProcess {
    #[serde(rename = "pid")]
    pub pid: i32,
    #[serde(rename = "ContainerId")]
    pub container_id: String,
    #[serde(rename = "name")]
    pub name: String,
    #[serde(rename = "MemoryUsage")]
    pub memory_usage: f32,
    #[serde(rename = "CpuUsage")]
    pub cpu_usage: f32,
    #[serde(rename = "IoActivity")]
    pub io_activity: i32,
    #[serde(rename = "action")]
    pub action: String,
    #[serde(rename = "timestamp")]
    pub timestamp: String,
}

// Implementación de comparación para ordenar procesos por uso de CPU y memoria
impl Eq for Process {}

impl Ord for Process {
    fn cmp(&self, other: &Self) -> std::cmp::Ordering {
        self.cpu_usage
            .partial_cmp(&other.cpu_usage)
            .unwrap_or(std::cmp::Ordering::Equal)
            .then_with(|| {
                self.memory_usage
                    .partial_cmp(&other.memory_usage)
                    .unwrap_or(std::cmp::Ordering::Equal)
                    .then_with(|| {
                        self.disk_usage
                            .partial_cmp(&other.disk_usage)
                            .unwrap_or(std::cmp::Ordering::Equal)
                            .then_with(|| {
                                self.io_activity
                                    .partial_cmp(&other.io_activity)
                                    .unwrap_or(std::cmp::Ordering::Equal)
                            })
                    })
            })
    }
}

impl PartialOrd for Process {
    fn partial_cmp(&self, other: &Self) -> Option<std::cmp::Ordering> {
        Some(self.cmp(other))
    }
}