import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Check, Sparkles, Gift } from "lucide-react";

export const Pricing = () => {
  const telegramBotUrl = "https://t.me/YourSplitBot"; // Reemplazar con tu bot

  const features = [
    "Gastos ilimitados",
    "Grupos ilimitados",
    "Miembros ilimitados",
    "División automática",
    "Historial completo",
    "Balance grupal",
    "Comandos intuitivos",
    "Seguridad de nivel empresarial",
    "Encriptación de datos",
    "Sin anuncios",
    "Sin tracking",
    "Código abierto",
  ];

  return (
    <section className="py-20 bg-gradient-to-br from-slate-900 via-slate-800 to-slate-900">
      <div className="container mx-auto px-4">
        <div className="text-center mb-16">
          <div className="inline-flex items-center gap-2 bg-green-500/20 px-4 py-2 rounded-full mb-6">
            <Gift className="h-5 w-5 text-green-400" />
            <span className="text-green-400 font-medium">100% Gratuito</span>
          </div>
          
          <h2 className="text-4xl lg:text-5xl font-bold text-white mb-6">
            Sin planes, sin costos, 
            <span className="bg-gradient-to-r from-[#0088cc] to-cyan-400 bg-clip-text text-transparent"> sin límites</span>
          </h2>
          <p className="text-xl text-slate-300 max-w-2xl mx-auto">
            Creemos que dividir gastos debería ser gratis. Por eso SplitBot no tiene planes pagos, ni anuncios, ni trucos.
          </p>
        </div>
        
        <div className="max-w-2xl mx-auto">
          <Card className="bg-gradient-to-br from-slate-800 to-slate-900 border-2 border-[#0088cc] shadow-2xl shadow-cyan-500/20 p-8 relative overflow-hidden">
            {/* Decoración */}
            <div className="absolute top-0 right-0 w-40 h-40 bg-gradient-to-br from-[#0088cc]/20 to-transparent rounded-full blur-3xl" />
            
            <div className="absolute -top-4 left-1/2 transform -translate-x-1/2 bg-gradient-to-r from-[#0088cc] to-cyan-500 text-white px-6 py-2 rounded-full flex items-center gap-2 shadow-lg">
              <Sparkles className="h-4 w-4" />
              <span className="font-semibold">Gratis para siempre</span>
            </div>
            
            <CardHeader className="p-0 mb-8 text-center pt-4">
              <CardTitle className="text-3xl font-bold text-white mb-2">SplitBot</CardTitle>
              <div className="mb-4">
                <span className="text-6xl font-bold text-[#0088cc]">$0</span>
                <span className="text-slate-400 ml-2 text-xl">/mes</span>
              </div>
              <p className="text-slate-300">Todo lo que necesitas para dividir gastos. Sin excepciones.</p>
            </CardHeader>
            
            <CardContent className="p-0">
              <div className="grid sm:grid-cols-2 gap-4 mb-8">
                {features.map((feature, index) => (
                  <div key={index} className="flex items-center gap-3">
                    <div className="bg-green-500/20 p-1 rounded-full">
                      <Check className="h-4 w-4 text-green-400" />
                    </div>
                    <span className="text-slate-200">{feature}</span>
                  </div>
                ))}
              </div>
              
              <Button 
                className="w-full bg-gradient-to-r from-[#0088cc] to-cyan-500 hover:from-[#0077b5] hover:to-cyan-600 text-white text-lg py-6 rounded-xl shadow-lg shadow-cyan-500/25"
                size="lg"
                onClick={() => window.open(telegramBotUrl, '_blank')}
              >
                Comenzar ahora — Es gratis
              </Button>
            </CardContent>
          </Card>
        </div>
        
        {/* Por qué es gratis */}
        <div className="mt-20 text-center max-w-3xl mx-auto">
          <h3 className="text-2xl font-bold text-white mb-6">¿Cómo puede ser gratis?</h3>
          <div className="grid md:grid-cols-3 gap-6">
            <div className="bg-slate-800/50 rounded-xl p-6">
              <div className="text-3xl mb-3">🤖</div>
              <h4 className="text-white font-semibold mb-2">Telegram es gratis</h4>
              <p className="text-slate-400 text-sm">La API de Telegram no tiene costos por mensaje enviado.</p>
            </div>
            <div className="bg-slate-800/50 rounded-xl p-6">
              <div className="text-3xl mb-3">☁️</div>
              <h4 className="text-white font-semibold mb-2">AWS Free Tier</h4>
              <p className="text-slate-400 text-sm">Lambda + DynamoDB gratis para bajo/medio tráfico.</p>
            </div>
            <div className="bg-slate-800/50 rounded-xl p-6">
              <div className="text-3xl mb-3">💚</div>
              <h4 className="text-white font-semibold mb-2">Open Source</h4>
              <p className="text-slate-400 text-sm">Proyecto de código abierto mantenido por la comunidad.</p>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
};
