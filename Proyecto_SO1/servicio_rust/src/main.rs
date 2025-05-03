mod process;
mod analyzer;
mod request;
mod init;
mod utils;

use utils::{read_proc_file, parser_proc_to_struct};
use analyzer::{analyzer, print_sistem};
use std::env;
use init::{start_cronjob, stop_cronjob, start_logs_server, get_logs_id, start_module, stop_module, stop_logs_server};
use tokio;
//use crate::request::grafica;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;
use ctrlc;
use tokio::runtime::Runtime;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Obtener el directorio actual y construir rutas
    let cwd = env::current_dir()?;
    let parent = cwd.parent().unwrap().display().to_string();

    let path_cronjob = format!("{}/script/cronjob.sh", parent);
    let path_docker_compose = format!("{}/pythonLogs/docker-compose.yml", &parent);

    // Iniciar el cronjob
    start_cronjob(&path_cronjob);

    println!("Iniciando el contenedor de logs...");
    start_logs_server(&path_docker_compose);

    // Esperar a que el contenedor de logs esté listo
    std::thread::sleep(std::time::Duration::from_secs(10));

    // Obtener el ID del contenedor de logs
    let id_container_logs = Arc::new(get_logs_id("pythonlogs_fastapi-app"));
    println!("Se creó el contenedor de logs con el ID: {}", id_container_logs);

    // Iniciar el módulo
    start_module();

    // Configurar el manejador de señales (Ctrl+C)
    let running = Arc::new(AtomicBool::new(true));
    let r = running.clone();
    let id_container_logs_clone = Arc::clone(&id_container_logs);

    ctrlc::set_handler(move || {
        println!("Deteniendo el funcionamiento del proyecto...");

        // Realizar la última lectura y análisis
        let json_str = match read_proc_file("sysinfo_202200349") {
            Ok(content) => content,
            Err(e) => {
                eprintln!("Error al leer el archivo sysinfo_202200349: {}", e);
                return;
            }
        };

        let system_info = match parser_proc_to_struct(&json_str) {
            Ok(info) => info,
            Err(e) => {
                eprintln!("Error al parsear el archivo sysinfo_202200349: {}", e);
                return;
            }
        };

        // Ejecutar el analyzer para generar el último log
        let rt = Runtime::new().unwrap();
        if let Err(e) = rt.block_on(analyzer(system_info, &id_container_logs_clone)) {
            eprintln!("Error al ejecutar el analyzer: {}", e);
        }

        // Detener el cronjob, módulo y servidor de logs
        stop_cronjob();
        println!("Cronjob detenido correctamente");
        stop_module();
        stop_logs_server(&path_docker_compose);
        println!("Servidor de logs detenido correctamente");

        // Indicar que el bucle principal debe detenerse
        r.store(false, Ordering::SeqCst);

        // Esperar un momento antes de salir
        std::thread::sleep(std::time::Duration::from_secs(10));
    }).expect("Error al configurar el manejador de señales");

    // Bucle principal
    while running.load(Ordering::SeqCst) {
        println!("Analizando contenedores...");

        // Leer y parsear el archivo sysinfo_202200349
        let json_str = match read_proc_file("sysinfo_202200349") {
            Ok(content) => content,
            Err(e) => {
                continue; // Saltar a la siguiente iteración del bucle
            }
        };
        let system_info = parser_proc_to_struct(&json_str);

        match system_info {
            Ok(info) => {
                print_sistem(&info.metrics);
                analyzer(info, &id_container_logs).await?;
            }
            Err(e) => {
                eprintln!("Error al parsear el archivo sysinfo_202200349: {}", e);
                continue; // Saltar a la siguiente iteración del bucle
            }
        }

        // Esperar 10 segundos antes de la siguiente iteración
        std::thread::sleep(std::time::Duration::from_secs(10));
    }

    println!("Fin del programa");
    Ok(())
}