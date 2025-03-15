use std::process::{Stdio};

pub fn start_module() -> std::process::Output{
    let output = std::process::Command::new("sudo")
        .arg("insmod")
        .arg("../modulo_kernel/sysinfo_module.ko")
        .output()
        .expect("Error al cargar el modulo");
    println!("Modulo cargado correctamente");
    output
}

pub fn start_cronjob(path: &str) -> std::process::Child {
    let output = std::process::Command::new("sh")
        .arg(path)
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .spawn()
        .expect("Error al iniciar el cronjob");
    output
}

pub fn stop_cronjob() -> std::process::Output {
    let output = std::process::Command::new("crontab")
        .arg("-r")
        .output()
        .expect("Error al detener el cronjob");
    println!("Cronjob detenido correctamente");
    output
}

// Iniciar el servidor de logs
pub fn start_logs_server(path: &str) -> std::process::Child {
    let output = std::process::Command::new("docker-compose")
        .arg("-f")
        .arg(path)
        .arg("up")
        .arg("-d")
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .spawn()
        .expect("Error al iniciar el servidor de logs");
    output
}

// Obtener el ID del contenedor de logs
pub fn get_logs_id(container_name: &str) -> String {
    let output = std::process::Command::new("docker")
        .arg("ps")
        .arg("--format")
        .arg("{{.ID}}")
        .arg("--filter")
        .arg(&format!("ancestor={}", container_name.to_string()))
        .output()
        .expect("Error al obtener el ID del contenedor de logs");

        let output_str = std::str::from_utf8(&output.stdout).expect("Error al convertir el output a string");

        let id_container_logs = output_str.trim().to_string();

        println!("ID del contenedor de logs: {}", &id_container_logs);

        return id_container_logs;
}

use std::process::Command;

pub fn get_container_name_by_id(container_id: &str) -> String {
    let output = Command::new("docker")
        .arg("ps")
        .arg("--filter")
        .arg(&format!("id={}", container_id))
        .arg("--format")
        .arg("{{.Names}}")
        .output()
        .expect("Error al obtener el nombre del contenedor");

    // Convertimos el resultado a String y eliminamos los saltos de línea
    let output_str = String::from_utf8_lossy(&output.stdout).trim().to_string();

    println!("Nombre del contenedor: {}", &output_str);

    output_str
}

//detener el modulo
pub fn stop_module() -> std::process::Output{
    let output = std::process::Command::new("sudo")
        .arg("rmmod")
        .arg("sysinfo_module")
        .output()
        .expect("Error al detener el modulo");
    println!("Modulo detenido correctamente");
    output
}

//Detener el contenedor de logs
pub fn stop_logs_server(path: &str) -> std::process::Output{
    let output = std::process::Command::new("docker-compose")
        .arg("-f")
        .arg(path)
        .arg("down")
        .output()
        .expect("Error al detener el contenedor de logs");
    println!("Contenedor de logs detenido correctamente");
    output
}