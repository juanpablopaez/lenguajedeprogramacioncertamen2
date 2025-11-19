import matplotlib.pyplot as plt
import numpy as np

tiempos_medidos = [241.331, 19.863, 8.721, 5.730]
Sucursales = [1, 4, 10, 20]
baseline = tiempos_medidos[0]
speedup_real = [baseline / t for t in tiempos_medidos]
speedup_ideal = Sucursales
plt.figure(figsize=(10, 6))
worker_labels = [f'{w} Sucursales' for w in Sucursales]
plt.bar(worker_labels, speedup_real, color='royalblue', label='Speedup Real (Medido)')
plt.plot(worker_labels, speedup_ideal, color='red', linestyle='--', marker='o', linewidth=2, label='Escalabilidad Lineal Ideal')
plt.title('Análisis de Escalabilidad (Speedup)', fontsize=16, fontweight='bold')
plt.xlabel('Número de Sucursales (Hilos)', fontsize=12)
plt.ylabel('Speedup (Mejora x Veces)', fontsize=12)
plt.legend(fontsize=11)
plt.grid(axis='y', linestyle='--', alpha=0.7) 
for i, speedup in enumerate(speedup_real):
    plt.text(i, speedup + 0.1, f'{speedup:.2f}x', ha='center', color='black', fontweight='bold')
plt.tight_layout() 
plt.savefig('grafico_speedup.png')
