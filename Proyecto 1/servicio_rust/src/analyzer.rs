use crate::process::{SystemInfo, Process, LogProcess, SystemMetrics};
use crate::request::{send_process};
use std::error::Error;
use chrono::{DateTime, Utc, Local};
use std::process::{Command};
use std::collections::HashMap;
use std::process::Stdio;


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

pub fn get_container_creation_date(container_id: &str) -> Result<String, Box<dyn std::error::Error>> {
    // Ejecutamos el comando docker inspect para obtener la fecha de creación
    let output = Command::new("docker")
        .arg("inspect")
        .arg("--format")
        .arg("{{.State.StartedAt}}")
        .arg(container_id)
        .output()?;

    // Convertimos el resultado a string
    let output_str = std::str::from_utf8(&output.stdout)?;

    // Eliminamos espacios en blanco adicionales
    let creation_date = output_str.trim().to_string();

    // Devolvemos la fecha de creación del contenedor
    Ok(creation_date)
}


pub async fn analyzer(system_info: SystemInfo, id_container_logs: &str) -> Result<(), Box<dyn Error>> {
    let mut lista_procesos: Vec<Process> = system_info.processes;

    // Eliminar el contenedor de logs
    lista_procesos.retain(|proceso| !proceso.container_id.starts_with(id_container_logs));

    // Agrupar contenedores por tipo
    let mut contenedores_por_tipo: HashMap<&str, Vec<Process>> = HashMap::new();
    contenedores_por_tipo.insert("cpu", Vec::new());
    contenedores_por_tipo.insert("io", Vec::new());
    contenedores_por_tipo.insert("ram", Vec::new());

    for proceso in lista_procesos {
        let nombre = get_container_name_by_id(&proceso.container_id); 
        let creation_date = get_container_creation_date(&proceso.container_id).ok();
        let proceso_con_fecha = Process { creation_date, ..proceso };

        if nombre.starts_with("cpu_") {
            contenedores_por_tipo.get_mut("cpu").unwrap().push(proceso_con_fecha);
        } else if nombre.starts_with("io_") {
            contenedores_por_tipo.get_mut("io").unwrap().push(proceso_con_fecha);
        } else if nombre.starts_with("ram_") {
            contenedores_por_tipo.get_mut("ram").unwrap().push(proceso_con_fecha);
        }
    }

    let mut lista_logs: Vec<LogProcess> = Vec::new();
    let now_utc: DateTime<Utc> = Utc::now();
    let now_gt: DateTime<Local> = now_utc.with_timezone(&Local::now().timezone());
    let formatted_date = now_gt.to_rfc3339();

    // Eliminar contenedores sobrantes
    for (tipo, lista) in contenedores_por_tipo.iter_mut() {
        if lista.len() > 1 {
            lista.sort_by(|a, b| b.creation_date.cmp(&a.creation_date));
            let eliminados = lista.split_off(1); // Elimina todos excepto el primero

            // Imprimir contenedores que se eliminarán
            print_container_table(&format!("Contenedores {} a eliminar", tipo), &eliminados, "Borrar");

            for proceso in eliminados {
                // Detener y eliminar contenedor
                stop_and_remove_container(&proceso.container_id);
                lista_logs.push(LogProcess {
                    pid: proceso.pid,
                    container_id: proceso.container_id.clone(),
                    name: proceso.name.clone(),
                    memory_usage: proceso.memory_usage,
                    cpu_usage: proceso.cpu_usage,
                    io_activity: proceso.io_activity,
                    action: "borrar".to_string(),
                    timestamp: formatted_date.clone(),
                });
            }

            // Imprimir contenedores que se mantienen
            print_container_table(&format!("Contenedores {} que se mantienen", tipo), lista, "Mantener");
        }
    }

    // Enviar logs al servidor
    let end_url: &str = "logs";
    println!("Enviando procesos al servidor");
    send_process(lista_logs, end_url).await?;

    Ok(())
}

fn print_container_table(title: &str, containers: &[Process], action: &str) {
    println!();
    println!("╔══════════════════════════════════════════════════════════════════════════════════════════════════════════════╗");
    println!("║ {:^98} ║", title);
    println!("╠════════════╦══════════════════════╦══════════════════════╦════════════╦════════════╦════════════╦════════════╦════════════╣");
    println!("║ {:<10} │ {:<20} │ {:<20} │ {:<10.2} │ {:<10.2} │ {:<10.2} │ {:<10.2} │ {:<10} ║", 
             "PID", "Container ID", "Name", "CPU %", "Memory %", "Disk Usage", "I/O Activity", "Action");
    println!("╠════════════╬══════════════════════╬══════════════════════╬════════════╬════════════╬════════════╬════════════╬════════════╣");

    for container in containers {
        println!(
            "║ {:<10} │ {:<20} │ {:<20} │ {:>8.2} │ {:>8.2} │ {:>8.2} │ {:>8.2} │ {:<10} ║",
            container.pid,
            container.container_id.get(..20).unwrap_or(&container.container_id).to_string(),
            container.name,
            container.cpu_usage,
            container.memory_usage,
            container.disk_usage,
            container.io_activity,
            action
        );
    }

    println!("╚════════════╩══════════════════════╩══════════════════════╩════════════╩════════════╩════════════╩════════════╩════════════╝");
    println!();
}


pub fn stop_and_remove_container(container_id: &str) -> Result<(), Box<dyn std::error::Error>> {
    // Verificar si el contenedor existe
    let check_output = Command::new("docker")
        .arg("ps")
        .arg("-a")
        .arg("--filter")
        .arg(format!("id={}", container_id))
        .arg("--format")
        .arg("{{.ID}}")
        .output()?;

    if check_output.stdout.is_empty() {
        return Err(format!("El contenedor con ID {} no existe", container_id).into());
    }

    // Detener el contenedor
    let stop_status = Command::new("docker")
        .arg("stop")
        .arg(container_id)
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .status()?;

    if !stop_status.success() {
        return Err(format!("Error al detener el contenedor {}", container_id).into());
    }

    println!("Contenedor {} detenido correctamente", container_id);

    // Eliminar el contenedor
    let remove_status = Command::new("docker")
        .arg("rm")
        .arg(container_id)
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .status()?;

    if !remove_status.success() {
        return Err(format!("Error al eliminar el contenedor {}", container_id).into());
    }

    println!("Contenedor {} eliminado correctamente", container_id);

    Ok(())
}

pub fn print_sistem(sisteminfo: &SystemMetrics) {
    println!();
    println!(" ╔═════════════════╦═══════════════════╦══════════════════════════════════════╗");
    println!(" ║ RAM Total (MB)  ║ RAM Libre (MB)    ║ RAM Usada (MB)  ║  % Uso CPU (MB)    ║");
    println!(" ╠═════════════════╬═══════════════════╬══════════════════════════════════════╣");
    println!(" ║ {:<14}  ║ {:<16}  ║ {:<16}  ║ {:<16} ║", sisteminfo.total_memory, sisteminfo.free_memory, sisteminfo.used_memory, sisteminfo.cpu_usage);
    println!(" ╚═════════════════╩═══════════════════╩══════════════════════════════════════╝");
    println!();
}