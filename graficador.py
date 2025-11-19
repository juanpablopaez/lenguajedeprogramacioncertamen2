import pandas as pd
import matplotlib.pyplot as plt
import matplotlib.ticker as ticker

file_path = 'simulacion_log.csv'
output_filename = 'Grafico_7.png'
try:
    df = pd.read_csv(file_path)
    df['timestamp_lvt'] = pd.to_numeric(df['timestamp_lvt'], errors='coerce')
    df['event_index'] = df.reset_index().index
    fig, ax = plt.subplots(figsize=(15, 8))
    worker_ids = sorted(df['sucursal_id'].unique())
    for worker_id in worker_ids:
        worker_df = df[df['sucursal_id'] == worker_id]
        ax.plot(
            worker_df['event_index'], 
            worker_df['timestamp_lvt'], 
            label=f'Sucursal {worker_id}', 
            marker='o',  
            markersize=3, 
            linestyle='-' 
        )
    ax.set_title('Progresión del Timestamp LVT por Sucursal', fontsize=16)
    ax.set_xlabel('Índice de Evento (Secuencia en el Log)', fontsize=12)
    ax.set_ylabel('Timestamp LVT', fontsize=12)
    ax.legend(title='ID Sucursal', bbox_to_anchor=(1.04, 1), loc='upper left')
    ax.grid(True, linestyle='--', alpha=0.6)
    ax.xaxis.set_major_formatter(ticker.FuncFormatter(lambda x, p: format(int(x), ',')))
    ax.yaxis.set_major_formatter(ticker.FuncFormatter(lambda x, p: format(int(x), ',')))
    plt.tight_layout(rect=[0, 0, 0.85, 1])
    plt.savefig(output_filename)
    plt.close(fig)
    print(f"¡'{output_filename}'!")
except FileNotFoundError:
    print(f"Error: '{file_path}'.")