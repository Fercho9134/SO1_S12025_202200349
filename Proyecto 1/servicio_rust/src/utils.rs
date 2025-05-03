use std::fs::File;
use std::io::{self, Read};
use std::path::Path;
use crate::process::{SystemInfo};

pub fn read_proc_file(file_name: &str)->io::Result<String>{
    let path = Path::new("/proc").join(file_name);
    let mut file = File::open(path)?;

    let mut content = String::new();
    file.read_to_string(&mut content)?;

    // Se retorna el contenido del archivo
    return Ok(content)
}

pub fn parser_proc_to_struct(json_str: &str)->Result<SystemInfo, serde_json::Error>{
    let system_info: SystemInfo = serde_json::from_str(json_str)?;
    return Ok(system_info)
}