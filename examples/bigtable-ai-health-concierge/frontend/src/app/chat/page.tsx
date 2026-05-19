'use client';

import { useState, useEffect, useRef } from 'react';
import { useRouter } from 'next/navigation';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts';

export const RechartsChart = ({ chart }: { chart: string }) => {
    try {
        const data = JSON.parse(chart);
        return (
            <div style={{ width: '100%', height: 300, minHeight: 300, minWidth: 600 }}>
                <ResponsiveContainer width="92%" height="100%">
                    <LineChart data={data} margin={{ top: 15, right: 50, bottom: 5, left: 0 }}>
                        <Line type="monotone" dataKey="value" stroke="#3b82f6" strokeWidth={2} isAnimationActive={false} />
                        <CartesianGrid stroke="#ccc" strokeDasharray="5 5" />
                        <XAxis dataKey="date" />
                        <YAxis />
                        <Tooltip />
                    </LineChart>
                </ResponsiveContainer>
            </div>
        );
    } catch (e) {
        return <div>Error parsing chart data</div>;
    }
};

interface Message {
    role: 'user' | 'agent';
    text: string;
}

export default function ChatPage() {
    const [messages, setMessages] = useState<Message[]>([]);
    const [input, setInput] = useState('');
    const [user, setUser] = useState<{ name: string, email: string } | null>(null);
    const [isTyping, setIsTyping] = useState(false);
    const messagesEndRef = useRef<HTMLDivElement>(null);
    const router = useRouter();

    useEffect(() => {
        // Check if user is logged in
        fetch('http://127.0.0.1:5000/api/user', { credentials: 'include' })
            .then(res => {
                if (res.status === 401) {
                    router.push('/');
                }
                return res.json();
            })
            .then(data => {
                if (data.name) {
                    setUser(data);
                    // Initial greeting
                    setMessages([
                        { role: 'agent', text: `Hello ${data.name.split(' ')[0]}! How can I help you today?` }
                    ]);
                }
            })
            .catch(() => router.push('/'));
    }, []);

    const scrollToBottom = () => {
        messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
    };

    useEffect(() => {
        scrollToBottom();
    }, [messages]);

    const sendMessage = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!input.trim() || isTyping) return;

        const userMessage: Message = { role: 'user', text: input };
        setMessages(prev => [...prev, userMessage]);
        setInput('');
        setIsTyping(true);

        try {
            const response = await fetch('http://127.0.0.1:5000/api/chat', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ message: input }),
                credentials: 'include'
            });
            const data = await response.json();
            setMessages(prev => [...prev, { role: 'agent', text: data.response }]);
        } catch (err) {
            setMessages(prev => [...prev, { role: 'agent', text: "Sorry, I'm having trouble connecting to the backend." }]);
        } finally {
            setIsTyping(false);
        }
    };

    const logout = () => {
        fetch('http://127.0.0.1:5000/api/logout', { credentials: 'include' })
            .then(() => router.push('/'));
    };

    if (!user) return <div style={{ padding: '2rem' }}>Authenticating...</div>;

    return (
        <>
            <header className="chat-header">
                <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
                    <img src="/cymbal.png" alt="Cymbal Logo" width="120" />
                    <h2 style={{ display: 'none' }}>Personal Health Concierge</h2>
                </div>
                <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
                    {/* <span style={{ fontSize: '0.875rem', color: '#94a3b8' }}>{user.email}</span> */}
                    <button onClick={logout} className="logout-btn">Logout</button>
                </div>
            </header>

            <div className="messages-container">
                {messages.map((msg, idx) => (
                    <div key={idx} className={`message ${msg.role}`}>
                        <ReactMarkdown
                            remarkPlugins={[remarkGfm]}
                            components={{
                                code({ node, inline, className, children, ...props }: any) {
                                    const match = /language-(\w+)/.exec(className || '');
                                    if (!inline && match && match[1] === 'recharts') {
                                        return <div className="recharts-chart-wrapper" style={{ width: '100%', maxWidth: '600px', border: '5px solid #ffffff', padding: '15px', backgroundColor: '#ffffff', borderRadius: '8px' }}><RechartsChart chart={String(children).replace(/\n$/, '')} /></div>;
                                    }
                                    return <code className={className} {...props}>{children}</code>;
                                },
                                pre({ node, ...props }: any) {
                                    const codeNode = node?.children?.find((c: any) => c.tagName === 'code');
                                    const classNames = codeNode?.properties?.className || [];
                                    const isRecharts = Array.isArray(classNames)
                                        ? classNames.includes('language-recharts')
                                        : String(classNames).includes('language-recharts');

                                    if (isRecharts) {
                                        return <div className="recharts-wrapper" style={{ background: 'transparent', padding: 0, margin: 0 }}>{props.children}</div>;
                                    }
                                    return <pre {...props}>{props.children}</pre>;
                                }
                            }}
                        >
                            {msg.text}
                        </ReactMarkdown>
                    </div>
                ))}
                {isTyping && <div className="message agent" style={{ opacity: 0.7 }}><span className="typing-dots">Typing</span></div>}
                <div ref={messagesEndRef} />
            </div>

            <form className="input-area" onSubmit={sendMessage}>
                <input style={{ fontSize: '0.925rem' }}
                    type="text"
                    className="chat-input"
                    placeholder="Type a message..."
                    value={input}
                    onChange={(e) => setInput(e.target.value)}
                />
                <button type="submit" className="send-btn" disabled={isTyping || !input.trim()} style={{ fontSize: '0.925rem' }}>
                    Send
                </button>
            </form>
        </>
    );
}
