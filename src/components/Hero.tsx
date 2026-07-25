import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Send, Calculator, Users, Shield } from "lucide-react";

export const Hero = () => {
  const telegramBotUrl = "https://t.me/YourSplitBot"; // Reemplazar con tu bot

  return (
    <section className="relative min-h-screen bg-gradient-to-br from-[#0088cc] via-[#0077b5] to-[#005a87] flex items-center justify-center overflow-hidden">
      {/* Patrón de fondo */}
      <div className="absolute inset-0 opacity-10">
        <div className="absolute inset-0" style={{
          backgroundImage: `url("data:image/svg+xml,%3Csvg width='60' height='60' viewBox='0 0 60 60' xmlns='http://www.w3.org/2000/svg'%3E%3Cg fill='none' fill-rule='evenodd'%3E%3Cg fill='%23ffffff' fill-opacity='0.4'%3E%3Cpath d='M36 34v-4h-2v4h-4v2h4v4h2v-4h4v-2h-4zm0-30V0h-2v4h-4v2h4v4h2V6h4V4h-4zM6 34v-4H4v4H0v2h4v4h2v-4h4v-2H6zM6 4V0H4v4H0v2h4v4h2V6h4V4H6z'/%3E%3C/g%3E%3C/g%3E%3C/svg%3E")`,
        }} />
      </div>
      
      <div className="absolute inset-0 bg-black/5"></div>
      
      <div className="container mx-auto px-4 py-20 relative z-10">
        <div className="grid lg:grid-cols-2 gap-12 items-center">
          <div className="text-center lg:text-left">
            {/* Badge */}
            <div className="inline-flex items-center gap-2 bg-white/20 backdrop-blur-sm px-4 py-2 rounded-full mb-6">
              <Shield className="h-4 w-4 text-white" />
              <span className="text-white text-sm font-medium">100% Seguro y Privado</span>
            </div>

            <h1 className="text-5xl lg:text-7xl font-bold text-white mb-6 leading-tight">
              Divide gastos en
              <span className="block bg-gradient-to-r from-white to-cyan-200 bg-clip-text text-transparent">
                Telegram
              </span>
            </h1>
            
            <p className="text-xl lg:text-2xl text-white/90 mb-8 leading-relaxed">
              El bot inteligente que gestiona todos los gastos compartidos de tu grupo. 
              <span className="font-semibold"> Sin apps adicionales, sin complicaciones, totalmente gratis.</span>
            </p>
            
            <div className="flex flex-col sm:flex-row gap-4 justify-center lg:justify-start mb-12">
              <Button 
                size="lg" 
                className="bg-white text-[#0088cc] hover:bg-white/90 text-lg px-8 py-6 rounded-full shadow-lg shadow-black/20 font-semibold"
                onClick={() => window.open(telegramBotUrl, '_blank')}
              >
                <Send className="mr-2 h-5 w-5" />
                Abrir en Telegram
              </Button>
              <Button 
                size="lg" 
                variant="outline" 
                className="border-2 border-white text-white hover:bg-white/10 text-lg px-8 py-6 rounded-full"
              >
                Ver Demo
              </Button>
            </div>
            
            <div className="flex flex-wrap justify-center lg:justify-start gap-8 text-white/90">
              <div className="flex items-center gap-2">
                <Users className="h-5 w-5" />
                <span>Grupos ilimitados</span>
              </div>
              <div className="flex items-center gap-2">
                <Calculator className="h-5 w-5" />
                <span>Cálculo automático</span>
              </div>
              <div className="flex items-center gap-2">
                <Send className="h-5 w-5" />
                <span>100% en Telegram</span>
              </div>
            </div>
          </div>
          
          <div className="relative">
            <Card className="bg-white/10 backdrop-blur-md p-8 shadow-2xl rounded-3xl border border-white/20">
              {/* Mock de chat de Telegram */}
              <div className="bg-[#17212b] rounded-2xl overflow-hidden shadow-lg">
                {/* Header del chat */}
                <div className="bg-[#232e3c] px-4 py-3 flex items-center gap-3">
                  <div className="w-10 h-10 rounded-full bg-gradient-to-br from-cyan-400 to-blue-500 flex items-center justify-center">
                    <span className="text-white font-bold">S</span>
                  </div>
                  <div>
                    <div className="text-white font-semibold">SplitBot</div>
                    <div className="text-gray-400 text-sm">bot</div>
                  </div>
                </div>
                
                {/* Mensajes */}
                <div className="p-4 space-y-3 min-h-[300px]">
                  {/* Mensaje del usuario */}
                  <div className="flex justify-end">
                    <div className="bg-[#2b5278] text-white px-4 py-2 rounded-2xl rounded-br-md max-w-[80%]">
                      /nuevo_gasto Cena 15000
                    </div>
                  </div>
                  
                  {/* Respuesta del bot */}
                  <div className="flex justify-start">
                    <div className="bg-[#182533] text-white px-4 py-3 rounded-2xl rounded-bl-md max-w-[85%]">
                      <div className="text-green-400 mb-1">✅ Gasto registrado</div>
                      <div className="space-y-1 text-sm">
                        <div>📝 <span className="font-semibold">Cena</span></div>
                        <div>💰 Monto: $15,000</div>
                        <div>👤 Creado por: Juan</div>
                        <div>🆔 ID: a1b2c3d4</div>
                      </div>
                      <div className="text-gray-400 text-xs mt-2">
                        Usa /dividir a1b2c3d4 para dividirlo
                      </div>
                    </div>
                  </div>

                  {/* Otro mensaje */}
                  <div className="flex justify-end">
                    <div className="bg-[#2b5278] text-white px-4 py-2 rounded-2xl rounded-br-md max-w-[80%]">
                      /dividir a1b2c3d4
                    </div>
                  </div>

                  {/* Respuesta división */}
                  <div className="flex justify-start">
                    <div className="bg-[#182533] text-white px-4 py-3 rounded-2xl rounded-bl-md max-w-[85%]">
                      <div className="mb-2">💰 <span className="font-semibold">Gasto dividido: Cena</span></div>
                      <div className="text-sm space-y-1">
                        <div>📊 Total: $15,000</div>
                        <div>👥 Participantes: 3</div>
                        <div className="text-cyan-400 font-semibold">💵 Por persona: $5,000</div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </Card>
            
            <div className="absolute -top-4 -right-4 bg-green-500 text-white px-6 py-3 rounded-full shadow-lg font-semibold animate-pulse">
              ¡Gratis!
            </div>
          </div>
        </div>
      </div>
    </section>
  );
};
