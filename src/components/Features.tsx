import { Card, CardContent } from "@/components/ui/card";
import { Send, Calculator, Receipt, Users, Clock, Shield, Lock, MessageCircle } from "lucide-react";

const features = [
  {
    icon: Send,
    title: "Telegram y WhatsApp",
    description: "Usalo desde el chat que ya tenés abierto: Telegram o WhatsApp Web, sin instalar otra app."
  },
  {
    icon: MessageCircle,
    title: "Grupos de WhatsApp",
    description: "Agregá el bot al grupo, hacé que cada integrante envíe /start y dividí gastos ahí mismo."
  },
  {
    icon: Calculator,
    title: "Cálculo automático",
    description: "Divide gastos automáticamente entre los participantes del grupo de forma equitativa."
  },
  {
    icon: Clock,
    title: "Recordatorios privados",
    description: "Desde el grupo, avisá por privado a quienes deben pagar. Cada persona habilita el contacto con /start."
  },
  {
    icon: Users,
    title: "Múltiples grupos",
    description: "Maneja gastos de diferentes grupos por separado: familia, amigos, trabajo, viajes."
  },
  {
    icon: Clock,
    title: "Historial completo",
    description: "Accede al historial completo de gastos y pagos con comandos simples."
  },
  {
    icon: Shield,
    title: "Seguridad de nivel empresarial",
    description: "Encriptación AES-256, WAF, rate limiting y protección contra DDoS incluidos."
  },
  {
    icon: Lock,
    title: "Privacidad garantizada",
    description: "Tus datos nunca se comparten. Cumplimos con las mejores prácticas de seguridad."
  },
  {
    icon: Receipt,
    title: "100% gratuito",
    description: "Sin costos ocultos ni planes premium para empezar a organizar los gastos de tu grupo."
  }
];

export const Features = () => {
  return (
    <section id="features" className="py-20 bg-slate-50">
      <div className="container mx-auto px-4">
        <div className="text-center mb-16">
          <h2 className="text-4xl lg:text-5xl font-bold text-slate-900 mb-6">
            Todo lo que necesitas para
            <span className="bg-gradient-to-r from-[#0088cc] to-[#00a8e8] bg-clip-text text-transparent"> dividir gastos</span>
          </h2>
          <p className="text-xl text-slate-600 max-w-3xl mx-auto">
            Desde gastos simples hasta viajes complicados, nuestro bot maneja todo tipo de divisiones de manera inteligente y segura.
          </p>
        </div>
        
        <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-6">
          {features.map((feature, index) => (
            <Card key={index} className="bg-white shadow-lg border-0 p-6 hover:shadow-xl hover:-translate-y-1 transition-all duration-300 group">
              <CardContent className="p-0">
                <div className="flex items-center mb-4">
                  <div className="bg-gradient-to-br from-[#0088cc] to-[#00a8e8] p-3 rounded-xl mr-4 group-hover:scale-110 transition-transform duration-300">
                    <feature.icon className="h-6 w-6 text-white" />
                  </div>
                </div>
                <h3 className="text-lg font-semibold text-slate-900 mb-2">{feature.title}</h3>
                <p className="text-slate-600 text-sm leading-relaxed">{feature.description}</p>
              </CardContent>
            </Card>
          ))}
        </div>
      </div>
    </section>
  );
};
